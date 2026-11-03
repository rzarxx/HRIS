package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/auth"
)

type deviceRow struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	EmployeeName   string  `json:"employee_name"`
	ApprovedByName *string `json:"approved_by_name"`
	RevokedReason  *string `json:"revoked_reason"`
}

func deviceHash(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

func register(c *client, name string) response {
	return c.do(http.MethodPost, "/v1/devices/register", map[string]any{
		"device_hash": deviceHash(name), "model": "Samsung A15 " + name, "os_version": "Android 14", "fcm_token": "rahasia-" + name,
	})
}

func pejuangClient(t *testing.T, srv *httptest.Server, subdivision string) (*client, account) {
	t.Helper()
	acc := activeAccount(t, "pejuang", subdivision)
	return login(t, srv, acc.email, seedPassword), acc
}

func superadminClient(t *testing.T, srv *httptest.Server, subdivision string) (*client, account) {
	t.Helper()
	acc := activeAccount(t, "pejuang", subdivision)
	exec(t, `UPDATE users SET role = 'superadmin' WHERE id = $1`, acc.userID)
	return login(t, srv, acc.email, seedPassword), acc
}

func TestDeviceLifecycle(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	hp, _ := pejuangClient(t, srv, "OPS-GDG")

	// Perangkat pertama langsung aktif, dan mendaftar ulang tidak membuat data baru.
	first := data[deviceRow](t, register(hp, "lama").want(t, http.StatusCreated))
	if first.Status != "approved" {
		t.Fatalf("perangkat pertama: %+v", first)
	}
	again := data[deviceRow](t, register(hp, "lama").want(t, http.StatusOK))
	if again.ID != first.ID || again.Status != "approved" {
		t.Errorf("daftar ulang: %+v", again)
	}
	if got := countAudit(t, "device.registered"); got != 1 {
		t.Errorf("audit device.registered = %d", got)
	}

	// Perangkat kedua menunggu persetujuan.
	second := data[deviceRow](t, register(hp, "baru").want(t, http.StatusCreated))
	if second.Status != "pending" {
		t.Fatalf("perangkat kedua: %+v", second)
	}
	mine := data[[]deviceRow](t, hp.do(http.MethodGet, "/v1/devices/me", nil).want(t, http.StatusOK))
	if len(mine) != 2 {
		t.Errorf("perangkat sendiri = %d", len(mine))
	}

	// Daftar peninjau tidak membocorkan hash dan token.
	res := sdm.do(http.MethodGet, "/v1/admin/devices?status=pending", nil).want(t, http.StatusOK)
	pending := data[[]deviceRow](t, res)
	if len(pending) != 1 || pending[0].ID != second.ID || pending[0].EmployeeName == "" {
		t.Errorf("daftar pending: %+v", pending)
	}
	body := string(res.body)
	if strings.Contains(body, deviceHash("baru")) || strings.Contains(body, "rahasia-") {
		t.Errorf("hash atau token bocor: %s", body)
	}
	sdm.do(http.MethodGet, "/v1/admin/devices?status=ngawur", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Sesi mobile perangkat lama ikut dicabut saat diganti.
	hpSession := insertDeviceSession(t, first.ID, hpUserID(t, first.ID))
	approved := data[deviceRow](t, sdm.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/approve", nil).want(t, http.StatusOK))
	if approved.Status != "approved" || approved.ApprovedByName == nil {
		t.Errorf("hasil setujui: %+v", approved)
	}
	assertSessionRevoked(t, hpSession)
	if got := countAudit(t, "device.approved"); got != 1 {
		t.Errorf("audit device.approved = %d", got)
	}
	var oldStatus, oldReason string
	if err := ownerPool.QueryRow(context.Background(), `SELECT status::text, revoked_reason FROM devices WHERE id = $1`, first.ID).Scan(&oldStatus, &oldReason); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "revoked" || oldReason != "replaced" {
		t.Errorf("perangkat lama: %s %s", oldStatus, oldReason)
	}

	// Setujui ulang tidak mengubah apa pun, dan perangkat yang dicabut tidak bisa dipakai lagi.
	sdm.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/approve", nil).want(t, http.StatusOK)
	if got := countAudit(t, "device.approved"); got != 1 {
		t.Errorf("setujui ulang menambah audit: %d", got)
	}
	register(hp, "lama").wantError(t, http.StatusForbidden, "DEVICE_REVOKED")
	sdm.do(http.MethodPost, "/v1/admin/devices/"+first.ID+"/approve", nil).wantError(t, http.StatusForbidden, "DEVICE_REVOKED")

	// Inti D32: setelah semua perangkat dicabut, HP berikutnya tetap harus disetujui.
	revoked := data[deviceRow](t, sdm.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/revoke", nil).want(t, http.StatusOK))
	if revoked.Status != "revoked" {
		t.Errorf("hasil cabut: %+v", revoked)
	}
	third := data[deviceRow](t, register(hp, "pencuri").want(t, http.StatusCreated))
	if third.Status != "pending" {
		t.Errorf("perangkat setelah dicabut harus pending: %+v", third)
	}

	// Validasi dan objek tak dikenal.
	hp.do(http.MethodPost, "/v1/devices/register", map[string]any{"device_hash": "pendek"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	const unknown = "00000000-0000-4000-8000-000000000004"
	sdm.do(http.MethodPost, "/v1/admin/devices/"+unknown+"/approve", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodPost, "/v1/admin/devices/"+unknown+"/revoke", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

// hpUserID mencari user pemilik perangkat untuk menyiapkan sesi mobile.
func hpUserID(t *testing.T, deviceID string) string {
	t.Helper()
	var id string
	err := ownerPool.QueryRow(context.Background(), `SELECT u.id::text FROM users u JOIN devices d ON d.employee_id = u.employee_id WHERE d.id = $1`, deviceID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertDeviceSession(t *testing.T, deviceID, userID string) string {
	t.Helper()
	var id string
	err := ownerPool.QueryRow(context.Background(), `
		INSERT INTO sessions (user_id, client, device_id, refresh_hash, idle_expires_at, absolute_expires_at)
		VALUES ($1, 'mobile', $2, $3, now() + interval '1 hour', now() + interval '1 day') RETURNING id::text`,
		userID, deviceID, deviceHash("sesi-"+deviceID)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertSessionRevoked(t *testing.T, sessionID string) {
	t.Helper()
	var revoked bool
	if err := ownerPool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM sessions WHERE id = $1`, sessionID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Error("sesi perangkat tidak dicabut")
	}
}

func TestRevokeDeviceRevokesSession(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	d := data[deviceRow](t, register(hp, "hilang").want(t, http.StatusCreated))
	session := insertDeviceSession(t, d.ID, acc.userID)

	sdm.do(http.MethodPost, "/v1/admin/devices/"+d.ID+"/revoke", nil).want(t, http.StatusOK)
	assertSessionRevoked(t, session)
	before := countAudit(t, "device.revoked")
	sdm.do(http.MethodPost, "/v1/admin/devices/"+d.ID+"/revoke", nil).want(t, http.StatusOK)
	if got := countAudit(t, "device.revoked"); got != before {
		t.Errorf("cabut ulang menambah audit: %d", got)
	}
}

// Akun peninjau yang dicuri tidak boleh bisa memasang HP baru sendiri.
func TestCannotApproveOwnDevice(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	register(sdm, "sdm-1").want(t, http.StatusCreated)
	second := data[deviceRow](t, register(sdm, "sdm-2").want(t, http.StatusCreated))
	sdm.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/approve", nil).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")

	// Peninjau lain boleh, dan mencabut perangkat sendiri juga boleh.
	admin, _ := superadminClient(t, srv, "OPS-GDG")
	admin.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/approve", nil).want(t, http.StatusOK)
	sdm.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/revoke", nil).want(t, http.StatusOK)
}

func TestOffboardingRevokesDevices(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	register(hp, "keluar-1").want(t, http.StatusCreated)
	register(hp, "keluar-2").want(t, http.StatusCreated)

	sdm.do(http.MethodPost, "/v1/admin/employees/"+acc.employeeID+"/deactivate", nil).want(t, http.StatusOK)
	var active int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM devices WHERE employee_id = $1 AND status <> 'revoked'`, acc.employeeID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("perangkat masih aktif setelah nonaktif: %d", active)
	}
}

// Super admin bisa semua kecuali keuangan (D34), dan SDM tidak bisa mengubah akunnya.
func TestSuperadminAllExceptFinance(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	admin, acc := superadminClient(t, srv, "IT-INF")
	hp, target := pejuangClient(t, srv, "OPS-GDG")
	register(hp, "a").want(t, http.StatusCreated)
	second := data[deviceRow](t, register(hp, "b").want(t, http.StatusCreated))

	// Akses luas di luar keuangan.
	for _, path := range []string{
		"/v1/admin/devices", "/v1/admin/office-locations", "/v1/admin/shifts", "/v1/admin/divisions", "/v1/admin/subdivisions",
		"/v1/admin/dashboard/summary", "/v1/admin/dashboard/org-chart", "/v1/admin/announcements",
		"/v1/admin/reports/attendance?year=2026&month=1",
	} {
		admin.do(http.MethodGet, path, nil).want(t, http.StatusOK)
	}
	admin.do(http.MethodPost, "/v1/admin/devices/"+second.ID+"/approve", nil).want(t, http.StatusOK)

	// Keuangan tertutup, dan gaji orang lain tidak pernah tampil.
	admin.do(http.MethodGet, "/v1/admin/reports/payroll-preview?year=2026&month=1", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	res := admin.do(http.MethodGet, "/v1/admin/employees/"+target.employeeID, nil).want(t, http.StatusOK)
	if strings.Contains(string(res.body), "base_salary") {
		t.Errorf("gaji orang lain terlihat: %s", res.body)
	}
	list := admin.do(http.MethodGet, "/v1/admin/employees?per_page=100", nil).want(t, http.StatusOK)
	if got := strings.Count(string(list.body), "base_salary"); got > 1 {
		t.Errorf("daftar memuat gaji orang lain: %d", got)
	}

	// Mengubah Pejuang boleh, mengubah gaji tidak.
	updated := data[employeeShift](t, admin.do(http.MethodPatch, "/v1/admin/employees/"+target.employeeID, map[string]any{"full_name": "Nama Baru Uji"}).want(t, http.StatusOK))
	if updated.FullName != "Nama Baru Uji" {
		t.Errorf("ubah nama: %+v", updated)
	}
	admin.do(http.MethodPatch, "/v1/admin/employees/"+target.employeeID, map[string]any{"base_salary": "9999999.00"}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	var salary string
	if err := ownerPool.QueryRow(context.Background(), `SELECT base_salary::text FROM employees WHERE id = $1`, target.employeeID).Scan(&salary); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(salary, "9999999") {
		t.Errorf("gaji berubah: %s", salary)
	}

	// Pejuang baru dari super admin bergaji 0, dan mengirim gaji ditolak.
	payload := func(email string) map[string]any {
		return map[string]any{"full_name": "Dibuat Admin", "email": email, "subdivision_id": subdivisionID(t, "OPS-LAP")}
	}
	made := data[createdEmployee](t, admin.do(http.MethodPost, "/v1/admin/employees", payload("dibuat.admin@pejuang.id")).want(t, http.StatusCreated))
	var madeSalary string
	if err := ownerPool.QueryRow(context.Background(), `SELECT base_salary::text FROM employees WHERE id = $1`, made.ID).Scan(&madeSalary); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(madeSalary, "0") {
		t.Errorf("gaji awal: %s", madeSalary)
	}
	withSalary := payload("dibuat.admin2@pejuang.id")
	withSalary["base_salary"] = "5000000.00"
	admin.do(http.MethodPost, "/v1/admin/employees", withSalary).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	// SDM tetap wajib mengisi gaji.
	sdm.do(http.MethodPost, "/v1/admin/employees", payload("dibuat.sdm@pejuang.id")).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Super admin boleh mengunci akun SDM, tetapi tidak boleh mengambil alihnya lewat tautan reset.
	var sdmUser string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE role = 'sdm' LIMIT 1`).Scan(&sdmUser); err != nil {
		t.Fatal(err)
	}
	admin.do(http.MethodPost, "/v1/admin/users/"+sdmUser+"/reset-password", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	admin.do(http.MethodPost, "/v1/admin/users/"+target.userID+"/reset-password", nil).want(t, http.StatusOK)
	admin.do(http.MethodPost, "/v1/admin/users/"+sdmUser+"/lock", nil).want(t, http.StatusNoContent)
	admin.do(http.MethodPost, "/v1/admin/users/"+sdmUser+"/unlock", nil).want(t, http.StatusNoContent)

	// SDM tidak bisa mengubah akun super admin dengan cara apa pun.
	sdm = loginSDM(t, srv)
	for _, action := range []string{"lock", "unlock", "revoke-sessions", "reset-password"} {
		sdm.do(http.MethodPost, "/v1/admin/users/"+acc.userID+"/"+action, nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	}
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+acc.employeeID, map[string]any{"full_name": "Diubah SDM"}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	sdm.do(http.MethodPost, "/v1/admin/employees/"+acc.employeeID+"/deactivate", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	admin.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	// Lokasi absen super admin hanya diubah super admin; daftarnya tetap utuh setelah SDM ditolak.
	loc := data[officeLocation](t, admin.do(http.MethodPost, "/v1/admin/office-locations", map[string]any{
		"name": "Kantor Admin", "latitude": -6.2, "longitude": 106.8, "radius_m": 100,
	}).want(t, http.StatusCreated))
	locationsPath := "/v1/admin/employees/" + acc.employeeID + "/locations"
	admin.do(http.MethodPut, locationsPath, map[string]any{"location_ids": []string{loc.ID}}).want(t, http.StatusOK)
	sdm.do(http.MethodPut, locationsPath, map[string]any{"location_ids": []string{}}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	if got := data[[]officeLocation](t, sdm.do(http.MethodGet, locationsPath, nil).want(t, http.StatusOK)); len(got) != 1 || got[0].ID != loc.ID {
		t.Errorf("lokasi super admin berubah: %+v", got)
	}
	// Pejuang biasa tetap bisa diatur SDM.
	sdm.do(http.MethodPut, "/v1/admin/employees/"+target.employeeID+"/locations", map[string]any{"location_ids": []string{loc.ID}}).want(t, http.StatusOK)

	// Role lain tetap tertutup, dan SDM tetap memegang gaji.
	login(t, srv, activeAccount(t, "pejuang", "OPS-LAP").email, seedPassword).do(http.MethodGet, "/v1/admin/devices", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	sdmView := sdm.do(http.MethodGet, "/v1/admin/employees/"+target.employeeID, nil).want(t, http.StatusOK)
	if !strings.Contains(string(sdmView.body), "base_salary") {
		t.Error("SDM harus melihat gaji")
	}
}

func TestResetSuperadminFromServer(t *testing.T) {
	srv := fresh(t, options{})
	_, acc := superadminClient(t, srv, "IT-INF")
	invite, err := auth.ResetSuperadmin(context.Background(), appPool, acc.email, time.Now())
	if err != nil || invite.Token == "" {
		t.Fatalf("reset: %v %v", invite, err)
	}
	newClient(t, srv).do(http.MethodPost, "/v1/auth/activate", map[string]any{"token": invite.Token, "password": "kata-sandi-baru-kuat-2"}).want(t, http.StatusNoContent) // gitleaks:allow
	login(t, srv, acc.email, "kata-sandi-baru-kuat-2").do(http.MethodGet, "/v1/admin/devices", nil).want(t, http.StatusOK)                                                // gitleaks:allow
	if _, err := auth.ResetSuperadmin(context.Background(), appPool, "sdm@pejuang.id", time.Now()); err == nil {
		t.Error("akun SDM tidak boleh direset lewat perintah ini")
	}
}

func TestGrantSuperadmin(t *testing.T) {
	srv := fresh(t, options{})
	ctx := context.Background()
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	hp.do(http.MethodGet, "/v1/admin/devices", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	invite, err := auth.GrantSuperadmin(ctx, appPool, acc.email, time.Now())
	if err != nil || invite != nil {
		t.Fatalf("grant: %v %v", invite, err)
	}
	if got := countAudit(t, "user.role_changed"); got != 1 {
		t.Errorf("audit user.role_changed = %d", got)
	}
	// Sesi lama tidak lagi berlaku di refresh; login baru membawa role baru.
	admin := login(t, srv, acc.email, seedPassword)
	admin.do(http.MethodGet, "/v1/admin/devices", nil).want(t, http.StatusOK)

	// Memberi ulang tidak mengubah apa pun.
	if _, err := auth.GrantSuperadmin(ctx, appPool, acc.email, time.Now()); err != nil {
		t.Errorf("grant ulang: %v", err)
	}
	if got := countAudit(t, "user.role_changed"); got != 1 {
		t.Errorf("grant ulang menambah audit: %d", got)
	}

	if err := auth.RevokeSuperadmin(ctx, appPool, acc.email); err != nil {
		t.Fatal(err)
	}
	back := login(t, srv, acc.email, seedPassword)
	back.do(http.MethodGet, "/v1/admin/devices", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	if err := auth.RevokeSuperadmin(ctx, appPool, acc.email); err == nil {
		t.Error("mencabut dari Pejuang biasa harus gagal")
	}

	// Role lain, akun tak dikenal, dan Pejuang nonaktif ditolak.
	if _, err := auth.GrantSuperadmin(ctx, appPool, "sdm@pejuang.id", time.Now()); err == nil {
		t.Error("akun SDM tidak boleh dijadikan super admin")
	}
	if _, err := auth.GrantSuperadmin(ctx, appPool, "tidak.ada@pejuang.id", time.Now()); err == nil {
		t.Error("email tak dikenal harus gagal")
	}

	// Akun yang belum aktif mendapat tautan aktivasi yang benar-benar berfungsi.
	sdm := loginSDM(t, srv)
	created := createEmployee(t, sdm, "admin.it@pejuang.id")
	invite, err = auth.GrantSuperadmin(ctx, appPool, "admin.it@pejuang.id", time.Now())
	if err != nil || invite == nil || invite.Token == "" {
		t.Fatalf("grant akun baru: %v %v", invite, err)
	}
	newClient := newClient(t, srv)
	newClient.do(http.MethodPost, "/v1/auth/activate", map[string]any{"token": invite.Token, "password": "kata-sandi-baru-kuat-1"}).want(t, http.StatusNoContent)
	login(t, srv, "admin.it@pejuang.id", "kata-sandi-baru-kuat-1").do(http.MethodGet, "/v1/admin/devices", nil).want(t, http.StatusOK) // gitleaks:allow

	if err := auth.RevokeSuperadmin(ctx, appPool, "admin.it@pejuang.id"); err != nil {
		t.Fatal(err)
	}
	sdm.do(http.MethodPost, "/v1/admin/employees/"+created.ID+"/deactivate", nil).want(t, http.StatusOK)
	if _, err := auth.GrantSuperadmin(ctx, appPool, "admin.it@pejuang.id", time.Now()); err == nil {
		t.Error("Pejuang nonaktif tidak boleh dijadikan super admin")
	}
}
