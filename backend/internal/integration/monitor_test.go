package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type monitorRow struct {
	EmployeeID  string   `json:"employee_id"`
	DivisionID  string   `json:"division_id"`
	Scheduled   bool     `json:"scheduled"`
	Status      *string  `json:"status"`
	ClockInAt   *string  `json:"clock_in_at"`
	LateMinutes int      `json:"late_minutes"`
	WorkMode    *string  `json:"work_mode"`
	InLatitude  *float64 `json:"in_latitude"`
}

func monitorFor(t *testing.T, c *client, query, employeeID string) monitorRow {
	t.Helper()
	for _, r := range data[[]monitorRow](t, c.do(http.MethodGet, "/v1/admin/attendance/monitor"+query, nil).want(t, http.StatusOK)) {
		if r.EmployeeID == employeeID {
			return r
		}
	}
	t.Fatalf("Pejuang %s tidak ada di monitor", employeeID)
	return monitorRow{}
}

// Monitor memuat semua Pejuang aktif, termasuk yang belum absen, dan mengikuti absen sungguhan.
func TestAttendanceMonitor(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	ctx := context.Background()

	var active int
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM employees WHERE deleted_at IS NULL AND status = 'active'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	all := data[[]monitorRow](t, f.sdm.do(http.MethodGet, "/v1/admin/attendance/monitor?date=2026-11-02", nil).want(t, http.StatusOK))
	if len(all) != active {
		t.Errorf("baris monitor %d, Pejuang aktif %d", len(all), active)
	}

	// Sebelum absen: terjadwal, tetapi belum ada status.
	before := monitorFor(t, f.sdm, "", f.account.employeeID)
	if before.Status != nil || !before.Scheduled || before.InLatitude != nil {
		t.Errorf("sebelum absen: %+v", before)
	}

	// Tanggal kosong memakai tanggal kerja saat ini.
	f.clock.Set(wibTime(2, 8, 30))
	f.clockIn(t).want(t, http.StatusCreated)
	after := monitorFor(t, f.sdm, "", f.account.employeeID)
	if after.Status == nil || *after.Status != "late" || after.LateMinutes != 30 || after.InLatitude == nil || after.WorkMode == nil || *after.WorkMode != "onsite" {
		t.Errorf("setelah absen: %+v", after)
	}
	if got := monitorFor(t, f.sdm, "?date=2026-11-03", f.account.employeeID); got.Status != nil {
		t.Errorf("tanggal lain ikut terbaca: %+v", got)
	}
	// Absen dini hari masih tanggal kerja sebelumnya, mengikuti cutoff.
	f.clock.Set(wibTime(3, 3, 0))
	if got := monitorFor(t, f.sdm, "", f.account.employeeID); got.Status == nil {
		t.Errorf("cutoff tidak diikuti: %+v", got)
	}

	// Sabtu bukan hari kerja shift, dan hari libur juga tidak terjadwal.
	if got := monitorFor(t, f.sdm, "?date=2026-11-07", f.account.employeeID); got.Scheduled {
		t.Errorf("Sabtu terjadwal: %+v", got)
	}
	exec(t, `INSERT INTO holidays (date, name) VALUES ('2026-11-10', 'Libur uji')`)
	if got := monitorFor(t, f.sdm, "?date=2026-11-10", f.account.employeeID); got.Scheduled {
		t.Errorf("hari libur terjadwal: %+v", got)
	}

	// Saringan divisi, dan respons tidak memuat gaji atau alasan risiko.
	res := f.sdm.do(http.MethodGet, "/v1/admin/attendance/monitor?division_id="+all[0].DivisionID, nil).want(t, http.StatusOK)
	for _, r := range data[[]monitorRow](t, res) {
		if r.DivisionID != all[0].DivisionID {
			t.Errorf("divisi lain ikut: %+v", r)
		}
	}
	for _, leaked := range []string{"base_salary", "risk_flags", "risk_score"} {
		if strings.Contains(string(res.body), leaked) {
			t.Errorf("respons memuat %s", leaked)
		}
	}

	// Pejuang nonaktif tidak ikut, dan validasi serta hak akses berlaku.
	f.sdm.do(http.MethodPost, "/v1/admin/employees/"+f.account.employeeID+"/deactivate", nil).want(t, http.StatusOK)
	for _, r := range data[[]monitorRow](t, f.sdm.do(http.MethodGet, "/v1/admin/attendance/monitor", nil).want(t, http.StatusOK)) {
		if r.EmployeeID == f.account.employeeID {
			t.Error("Pejuang nonaktif masih tampil")
		}
	}
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/monitor?date=kemarin", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/monitor?division_id=ngarang", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	login(t, f.srv, activeAccount(t, "pejuang", "OPS-LAP").email, seedPassword).do(http.MethodGet, "/v1/admin/attendance/monitor", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}
