package integration

import (
	"net/http"
	"testing"
)

type shiftRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type employeeShifts struct {
	DefaultShiftID *string    `json:"default_shift_id"`
	Shifts         []shiftRef `json:"shifts"`
}

// D61: satu Pejuang bisa punya beberapa shift dengan hari berbeda; setengah hari dihitung 0,5 di rekap.
func TestMultipleShiftsAndHalfDay(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	newShift := func(body map[string]any) shiftRow {
		return data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", body).want(t, http.StatusCreated))
	}
	weekday := newShift(map[string]any{"name": "Kantor", "start_time": "08:00", "end_time": "16:00", "late_tolerance_minutes": 10, "work_days": []int{1, 2, 3, 4, 5}})
	saturday := newShift(map[string]any{"name": "Sabtu", "start_time": "08:00", "end_time": "12:00", "late_tolerance_minutes": 10, "work_days": []int{6}, "kind": "half_day"})
	overlap := newShift(map[string]any{"name": "Jumat-Sabtu", "start_time": "13:00", "end_time": "17:00", "late_tolerance_minutes": 0, "work_days": []int{5, 6}})
	sdm.do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Salah", "start_time": "08:00", "end_time": "12:00", "late_tolerance_minutes": 0, "work_days": []int{1}, "kind": "seperempat",
	}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	acc := activeAccount(t, "pejuang", "OPS-GDG")
	path := "/v1/admin/employees/" + acc.employeeID
	got := data[employeeShifts](t, sdm.do(http.MethodPatch, path, map[string]any{"shift_ids": []string{weekday.ID, saturday.ID}}).want(t, http.StatusOK))
	if got.DefaultShiftID == nil || *got.DefaultShiftID != weekday.ID || len(got.Shifts) != 2 || got.Shifts[1].Kind != "half_day" {
		t.Fatalf("dua shift: %+v", got)
	}

	// Hari kerja bertabrakan ditolak.
	res := sdm.do(http.MethodPatch, path, map[string]any{"shift_ids": []string{weekday.ID, overlap.ID}}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	if res.fields()["shift_ids"] == nil {
		t.Fatalf("galat tabrakan: %s", res.body)
	}

	// Mengubah field lain tidak menyentuh daftar shift.
	got = data[employeeShifts](t, sdm.do(http.MethodPatch, path, map[string]any{"position": "Staf"}).want(t, http.StatusOK))
	if len(got.Shifts) != 2 {
		t.Fatalf("daftar shift berubah: %+v", got)
	}

	// Rekap: hadir di shift setengah hari dihitung 0,5 hari kerja.
	exec(t, `INSERT INTO attendances (employee_id, work_date, shift_id, status, clock_in_at, clock_out_at)
		VALUES ($1, '2020-02-03', $2, 'present', '2020-02-03 01:00+00', '2020-02-03 09:00+00'),
		       ($1, '2020-02-08', $3, 'present', '2020-02-08 01:00+00', '2020-02-08 05:00+00')`, acc.employeeID, weekday.ID, saturday.ID)
	rows := data[[]struct {
		EmployeeID string  `json:"employee_id"`
		WorkDays   float64 `json:"work_days"`
		Present    int     `json:"present"`
	}](t, sdm.do(http.MethodGet, "/v1/admin/reports/attendance?year=2020&month=2", nil).want(t, http.StatusOK))
	found := false
	for _, r := range rows {
		if r.EmployeeID == acc.employeeID {
			found = true
			if r.WorkDays != 1.5 || r.Present != 2 {
				t.Errorf("rekap: hari kerja=%v hadir=%d, ingin 1.5 dan 2", r.WorkDays, r.Present)
			}
		}
	}
	if !found {
		t.Fatal("Pejuang tidak ada di rekap")
	}

	// Klien lama yang mengirim default_shift_id mengganti seluruh daftar dengan satu shift.
	got = data[employeeShifts](t, sdm.do(http.MethodPatch, path, map[string]any{"default_shift_id": saturday.ID}).want(t, http.StatusOK))
	if len(got.Shifts) != 1 || got.Shifts[0].ID != saturday.ID {
		t.Fatalf("default_shift_id lama: %+v", got)
	}
}
