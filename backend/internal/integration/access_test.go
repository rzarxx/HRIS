package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type employeeRow struct {
	ID            string  `json:"id"`
	FullName      string  `json:"full_name"`
	SubdivisionID string  `json:"subdivision_id"`
	DivisionID    string  `json:"division_id"`
	Status        string  `json:"status"`
	BaseSalary    *string `json:"base_salary"`
	Account       *struct {
		UserID string `json:"user_id"`
	} `json:"account"`
}

type employeeMeta struct {
	Total int `json:"total"`
}

func listEmployees(t *testing.T, c *client) ([]employeeRow, int) {
	t.Helper()
	res := c.do(http.MethodGet, "/v1/admin/employees?per_page=100", nil).want(t, http.StatusOK)
	var parsed struct {
		Data []employeeRow `json:"data"`
		Meta employeeMeta  `json:"meta"`
	}
	if err := jsonUnmarshal(res.body, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Data, parsed.Meta.Total
}

func employeeInSubdivision(t *testing.T, code string) string {
	t.Helper()
	var id string
	err := ownerPool.QueryRow(context.Background(), `
		SELECT e.id::text FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
		WHERE s.code = $1 AND e.status = 'active' ORDER BY e.nip DESC LIMIT 1`, code).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Objek di luar scope dijawab 404, bukan 403 (MASTER.md §3.3 no. 4).
func TestScopeKadiv(t *testing.T) {
	srv := fresh(t, options{})
	kadiv := activeAccount(t, "kadiv", "OPS-LAP")
	c := login(t, srv, kadiv.email, seedPassword)

	rows, total := listEmployees(t, c)
	if total == 0 || len(rows) != total {
		t.Fatalf("daftar: %d dari %d", len(rows), total)
	}
	for _, e := range rows {
		if e.DivisionID != kadiv.division {
			t.Errorf("%s di luar divisi ikut terlihat", e.FullName)
		}
	}

	c.do(http.MethodGet, "/v1/admin/employees/"+employeeInSubdivision(t, "KEU-AKT"), nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	inside := data[employeeRow](t, c.do(http.MethodGet, "/v1/admin/employees/"+employeeInSubdivision(t, "OPS-GDG"), nil).want(t, http.StatusOK))
	if inside.BaseSalary != nil {
		t.Error("gaji orang lain tidak boleh terlihat oleh kadiv")
	}
	if inside.Account != nil {
		t.Error("data akun hanya untuk pemegang user:manage")
	}
	self := data[employeeRow](t, c.do(http.MethodGet, "/v1/admin/employees/"+kadiv.employeeID, nil).want(t, http.StatusOK))
	if self.BaseSalary == nil {
		t.Error("gaji sendiri harus terlihat")
	}

	// Permission yang tidak dimiliki kadiv.
	c.do(http.MethodPost, "/v1/admin/employees", map[string]any{}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	c.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	c.do(http.MethodGet, "/v1/admin/divisions", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	c.do(http.MethodGet, "/v1/admin/announcements", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	// Laporan presensi hanya divisinya; pratinjau upah memuat gaji sehingga ditolak (D15).
	report := data[[]struct {
		Division string `json:"division"`
	}](t, c.do(http.MethodGet, "/v1/admin/reports/attendance?year=2026&month=10", nil).want(t, http.StatusOK))
	if len(report) == 0 {
		t.Fatal("rekap kosong")
	}
	for _, r := range report {
		if r.Division != "Operasional" {
			t.Errorf("rekap memuat divisi %s", r.Division)
		}
	}
	c.do(http.MethodGet, "/v1/admin/reports/payroll-preview?year=2026&month=10", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	// Filter divisi lain tidak membuka data di luar scope.
	var otherDivision string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM divisions WHERE code = 'KEU'`).Scan(&otherDivision); err != nil {
		t.Fatal(err)
	}
	other := data[[]struct {
		Nip string `json:"nip"`
	}](t, c.do(http.MethodGet, "/v1/admin/reports/attendance?year=2026&month=10&division_id="+otherDivision, nil).want(t, http.StatusOK))
	if len(other) != 0 {
		t.Errorf("filter divisi lain mengembalikan %d baris", len(other))
	}
}

func TestScopeSubdiv(t *testing.T) {
	srv := fresh(t, options{})
	head := activeAccount(t, "subdiv", "OPS-GDG")
	c := login(t, srv, head.email, seedPassword)

	rows, _ := listEmployees(t, c)
	if len(rows) == 0 {
		t.Fatal("daftar kosong")
	}
	for _, e := range rows {
		if e.SubdivisionID != head.subdivision {
			t.Errorf("%s di luar subdiv ikut terlihat", e.FullName)
		}
	}
	c.do(http.MethodGet, "/v1/admin/employees/"+employeeInSubdivision(t, "OPS-DST"), nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	c.do(http.MethodGet, "/v1/admin/reports/attendance?year=2026&month=10", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}

// Scope mengikuti struktur organisasi di DB, bukan klaim JWT (MASTER.md §3.1).
func TestScopeFollowsOrganization(t *testing.T) {
	srv := fresh(t, options{})
	head := activeAccount(t, "subdiv", "OPS-GDG")
	c := login(t, srv, head.email, seedPassword)
	exec(t, `UPDATE subdivisions SET head_employee_id = NULL WHERE head_employee_id = $1`, head.employeeID)
	rows, total := listEmployees(t, c)
	if len(rows) != 0 || total != 0 {
		t.Errorf("setelah bukan kepala masih melihat %d Pejuang", total)
	}
}

func TestPejuangHasNoAdminAccess(t *testing.T) {
	srv := fresh(t, options{})
	pejuang := activeAccount(t, "pejuang", "OPS-GDG")
	c := login(t, srv, pejuang.email, seedPassword)
	c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)
	for _, path := range []string{
		"/v1/admin/employees", "/v1/admin/divisions", "/v1/admin/subdivisions", "/v1/admin/announcements",
		"/v1/admin/dashboard/org-chart", "/v1/admin/reports/attendance?year=2026&month=10",
	} {
		c.do(http.MethodGet, path, nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	}
	sdm := activeAccount(t, "kadiv", "SDM-PRS")
	c.do(http.MethodPost, "/v1/admin/users/"+sdm.userID+"/lock", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}

func TestManageAccounts(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	target := activeAccount(t, "pejuang", "OPS-GDG")
	session := login(t, srv, target.email, seedPassword)

	// Kunci: sesi mati dan tidak bisa masuk.
	sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/lock", nil).want(t, http.StatusNoContent)
	session.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": target.email, "password": seedPassword}).
		wantError(t, http.StatusLocked, "AUTH_ACCOUNT_LOCKED")
	sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/unlock", nil).want(t, http.StatusNoContent)
	session = login(t, srv, target.email, seedPassword)

	sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/revoke-sessions", nil).want(t, http.StatusNoContent)
	session.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	// Reset: sandi lama tidak berlaku, tautan baru menetapkan sandi.
	session = login(t, srv, target.email, seedPassword)
	activation := data[struct {
		Token string `json:"token"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/reset-password", nil).want(t, http.StatusOK))
	session.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": target.email, "password": seedPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/activate", map[string]any{"token": activation.Token, "password": newPassword}).
		want(t, http.StatusNoContent)
	login(t, srv, target.email, newPassword)

	// Undangan lama gugur saat undangan baru terbit.
	first := data[struct {
		Token string `json:"token"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/reset-password", nil).want(t, http.StatusOK))
	second := data[struct {
		Token string `json:"token"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/reset-password", nil).want(t, http.StatusOK))
	newClient(t, srv).do(http.MethodPost, "/v1/auth/activate", map[string]any{"token": first.Token, "password": newPassword}).
		wantError(t, http.StatusUnprocessableEntity, "ACTIVATION_USED")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/activate", map[string]any{"token": second.Token, "password": newPassword}).
		want(t, http.StatusNoContent)

	// Tidak boleh pada akun sendiri, dan akun tak dikenal dijawab 404.
	me := data[struct {
		UserID string `json:"user_id"`
	}](t, sdm.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	for _, action := range []string{"lock", "reset-password", "revoke-sessions"} {
		sdm.do(http.MethodPost, "/v1/admin/users/"+me.UserID+"/"+action, nil).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	}
	sdm.do(http.MethodPost, "/v1/admin/users/00000000-0000-4000-8000-000000000000/lock", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestOffboarding(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	target := activeAccount(t, "subdiv", "OPS-GDG")
	session := login(t, srv, target.email, seedPassword)

	sdm.do(http.MethodPost, "/v1/admin/employees/"+target.employeeID+"/deactivate", nil).want(t, http.StatusOK)
	session.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": target.email, "password": seedPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")

	var head *string
	if err := ownerPool.QueryRow(context.Background(), `SELECT head_employee_id::text FROM subdivisions WHERE id = $1`, target.subdivision).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if head != nil {
		t.Error("Pejuang nonaktif tidak boleh tetap menjadi kepala subdiv")
	}
	// Idempoten, dan riwayat tetap ada.
	sdm.do(http.MethodPost, "/v1/admin/employees/"+target.employeeID+"/deactivate", nil).want(t, http.StatusOK)
	if countAudit(t, "employee.deactivated") != 1 {
		t.Error("penonaktifan harus tercatat sekali")
	}

	// Tidak boleh menonaktifkan diri sendiri.
	me := data[struct {
		EmployeeID string `json:"employee_id"`
	}](t, sdm.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	sdm.do(http.MethodPost, "/v1/admin/employees/"+me.EmployeeID+"/deactivate", nil).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+me.EmployeeID, map[string]any{"status": "inactive"}).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
}

// audit_logs tidak bisa diubah oleh role yang dipakai API (MASTER.md §5.10).
func TestAuditLogIsAppendOnly(t *testing.T) {
	srv := fresh(t, options{})
	loginSDM(t, srv)
	ctx := context.Background()

	var count int
	if err := appPool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("audit kosong")
	}
	for _, stmt := range []string{`UPDATE audit_logs SET action = 'x'`, `DELETE FROM audit_logs`, `TRUNCATE audit_logs`} {
		_, err := appPool.Exec(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: ingin ditolak (42501), dapat %v", stmt, err)
		}
	}
	// Role aplikasi juga tidak boleh mengubah skema.
	if _, err := appPool.Exec(ctx, `DROP TABLE employees`); err == nil {
		t.Error("hris_app tidak boleh menghapus tabel")
	}
}

func TestAuditHasNoSecrets(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	createEmployee(t, sdm, "audit.rahasia@pejuang.id")
	rows, err := ownerPool.Query(context.Background(), `SELECT action, COALESCE(detail::text, '') FROM audit_logs`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var action, detail string
		if err := rows.Scan(&action, &detail); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{seedPassword, "password", "token"} {
			if strings.Contains(strings.ToLower(detail), secret) {
				t.Errorf("audit %s memuat %q: %s", action, secret, detail)
			}
		}
	}
}
