package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

type bulkResult struct {
	Created int `json:"created"`
	Skipped []struct {
		EmployeeID string `json:"employee_id"`
		Reason     string `json:"reason"`
	} `json:"skipped"`
}

func activeEmployeeIDs(t *testing.T, subdivisionID string) []string {
	t.Helper()
	rows, err := ownerPool.Query(context.Background(), `SELECT id::text FROM employees WHERE subdivision_id = $1 AND status = 'active' AND deleted_at IS NULL ORDER BY id`, subdivisionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// Izin massal tersimpan per Pejuang, dan yang tidak boleh diberi izin dilewati, bukan menggagalkan semuanya.
func TestBulkWorkArrangements(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	ids := activeEmployeeIDs(t, acc.subdivision)
	if len(ids) < 3 {
		t.Fatalf("subdiv uji butuh minimal 3 Pejuang, ada %d", len(ids))
	}
	base := map[string]any{"mode": "wfa", "starts_on": "2026-11-09", "ends_on": "2026-11-13", "reason": "Renovasi gudang"}
	sdm.do(http.MethodPost, "/v1/admin/employees/"+ids[0]+"/work-arrangements", base).want(t, http.StatusCreated)
	exec(t, `UPDATE users SET role = 'superadmin' WHERE employee_id = $1`, ids[1])

	body := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	bulk := func(extra map[string]any) response {
		return sdm.do(http.MethodPost, "/v1/admin/work-arrangements/bulk", body(extra))
	}

	got := data[bulkResult](t, bulk(map[string]any{"subdivision_id": acc.subdivision}).want(t, http.StatusOK))
	if got.Created != len(ids)-2 || len(got.Skipped) != 2 {
		t.Fatalf("hasil: %+v (Pejuang %d)", got, len(ids))
	}
	reasons := map[string]string{}
	for _, s := range got.Skipped {
		reasons[s.EmployeeID] = s.Reason
	}
	if reasons[ids[0]] != "overlap" || reasons[ids[1]] != "protected" {
		t.Errorf("dilewati: %v", reasons)
	}
	var stored int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM employee_work_arrangements w JOIN employees e ON e.id = w.employee_id WHERE e.subdivision_id = $1`, acc.subdivision).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != len(ids)-1 {
		t.Errorf("izin tersimpan = %d", stored)
	}
	if n := countAudit(t, "employee.work_arrangement_changed"); n != len(ids)-1 {
		t.Errorf("audit = %d", n)
	}

	// Diulang: semuanya bertabrakan, tidak ada yang dibuat dua kali.
	again := data[bulkResult](t, bulk(map[string]any{"subdivision_id": acc.subdivision}).want(t, http.StatusOK))
	if again.Created != 0 {
		t.Errorf("ulang: %+v", again)
	}

	// Akun sendiri dilewati.
	var sdmEmployee, sdmDivision string
	if err := ownerPool.QueryRow(context.Background(), `SELECT e.id::text, s.division_id::text FROM users u JOIN employees e ON e.id = u.employee_id JOIN subdivisions s ON s.id = e.subdivision_id WHERE u.email = $1`, sdmEmail).Scan(&sdmEmployee, &sdmDivision); err != nil {
		t.Fatal(err)
	}
	own := data[bulkResult](t, bulk(map[string]any{"division_id": sdmDivision, "starts_on": "2026-12-01", "ends_on": nil}).want(t, http.StatusOK))
	self := false
	for _, s := range own.Skipped {
		self = self || (s.EmployeeID == sdmEmployee && s.Reason == "self")
	}
	if !self || own.Created == 0 {
		t.Errorf("divisi sendiri: %+v", own)
	}

	for name, extra := range map[string]map[string]any{
		"tanpa unit":   {},
		"dua unit":     {"division_id": sdmDivision, "subdivision_id": acc.subdivision},
		"id salah":     {"subdivision_id": "bukan-uuid"},
		"unit kosong":  {"subdivision_id": "00000000-0000-0000-0000-000000000000"},
		"mode salah":   {"subdivision_id": acc.subdivision, "mode": "kantor"},
		"tanpa alasan": {"subdivision_id": acc.subdivision, "reason": " "},
	} {
		if res := bulk(extra); res.status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, res.status, res.body)
		}
	}

	hp, _ := pejuangClient(t, srv, "OPS-GDG")
	hp.do(http.MethodPost, "/v1/admin/work-arrangements/bulk", body(map[string]any{"subdivision_id": acc.subdivision})).
		wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}

// SDM bisa melihat upaya absen yang ditolak, yang tidak pernah muncul sebagai absen.
func TestAttendanceAttempts(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.clockIn(t).want(t, http.StatusCreated)
	exec(t, `INSERT INTO attendance_attempts (employee_id, purpose, outcome, reason_code, risk_score, risk_flags) VALUES ($1, 'clock_in', 'rejected', 'geofence_out', 0, '["accuracy_low"]')`, f.account.employeeID)

	type attempt struct {
		EmployeeID string   `json:"employee_id"`
		Outcome    string   `json:"outcome"`
		ReasonCode string   `json:"reason_code"`
		RiskFlags  []string `json:"risk_flags"`
		Purpose    string   `json:"purpose"`
	}
	today := time.Now().In(pgconv.WIB).Format("2006-01-02")
	all := data[[]attempt](t, f.sdm.do(http.MethodGet, "/v1/admin/attendance/attempts?date="+today, nil).want(t, http.StatusOK))
	if len(all) != 2 {
		t.Fatalf("upaya = %+v", all)
	}
	if all[0].Outcome != "rejected" || all[0].ReasonCode != "geofence_out" || len(all[0].RiskFlags) != 1 || all[0].EmployeeID != f.account.employeeID {
		t.Errorf("terbaru: %+v", all[0])
	}
	if all[1].Outcome != "accepted" || all[1].Purpose != "clock_in" {
		t.Errorf("diterima: %+v", all[1])
	}
	rejected := data[[]attempt](t, f.sdm.do(http.MethodGet, "/v1/admin/attendance/attempts?date="+today+"&outcome=rejected", nil).want(t, http.StatusOK))
	if len(rejected) != 1 {
		t.Errorf("saring ditolak: %+v", rejected)
	}
	yesterday := time.Now().In(pgconv.WIB).AddDate(0, 0, -1).Format("2006-01-02")
	if none := data[[]attempt](t, f.sdm.do(http.MethodGet, "/v1/admin/attendance/attempts?date="+yesterday, nil).want(t, http.StatusOK)); len(none) != 0 {
		t.Errorf("kemarin: %+v", none)
	}
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/attempts?outcome=lain", nil).want(t, http.StatusBadRequest)
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/attempts?date=31-12-2026", nil).want(t, http.StatusBadRequest)
	f.hp.do(http.MethodGet, "/v1/admin/attendance/attempts", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}
