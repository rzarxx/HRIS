package integration

import (
	"context"
	"net/http"
	"testing"
)

type employeeShift struct {
	ID               string  `json:"id"`
	FullName         string  `json:"full_name"`
	DefaultShiftID   *string `json:"default_shift_id"`
	DefaultShiftName *string `json:"default_shift_name"`
}

func TestEmployeeDefaultShift(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	shift := data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Malam", "start_time": "22:00", "end_time": "06:00", "late_tolerance_minutes": 15, "work_days": []int{1, 2, 3},
	}).want(t, http.StatusCreated))

	// Dibuat tanpa shift: belum ditentukan, bukan otomatis shift pertama.
	created := createEmployee(t, sdm, "shift.baru@pejuang.id")
	path := "/v1/admin/employees/" + created.ID
	got := data[employeeShift](t, sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK))
	if got.DefaultShiftID != nil || got.DefaultShiftName != nil {
		t.Errorf("Pejuang baru sudah punya shift: %+v", got)
	}

	// Menetapkan shift memuat namanya dan tercatat di audit.
	before := countAudit(t, "employee.updated")
	got = data[employeeShift](t, sdm.do(http.MethodPatch, path, map[string]any{"default_shift_id": shift.ID}).want(t, http.StatusOK))
	if got.DefaultShiftID == nil || *got.DefaultShiftID != shift.ID || got.DefaultShiftName == nil || *got.DefaultShiftName != "Malam" {
		t.Errorf("shift tidak tersimpan: %+v", got)
	}
	if after := countAudit(t, "employee.updated"); after != before+1 {
		t.Errorf("audit employee.updated %d, ingin %d", after, before+1)
	}
	var detail *string
	err := ownerPool.QueryRow(context.Background(), `
		SELECT detail->>'default_shift_id' FROM audit_logs
		WHERE action = 'employee.updated' AND entity_id = $1 ORDER BY created_at DESC LIMIT 1`, created.ID).Scan(&detail)
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil || *detail != shift.ID {
		t.Errorf("detail audit tidak memuat shift: %v", detail)
	}

	// Field yang tidak dikirim tetap nilai lamanya.
	got = data[employeeShift](t, sdm.do(http.MethodPatch, path, map[string]any{"full_name": "Rudi Hartanto"}).want(t, http.StatusOK))
	if got.FullName != "Rudi Hartanto" || got.DefaultShiftID == nil {
		t.Errorf("PATCH nama mengubah shift: %+v", got)
	}

	// Muncul juga di daftar Pejuang.
	rows := data[[]employeeShift](t, sdm.do(http.MethodGet, "/v1/admin/employees?q=Rudi+Hartanto", nil).want(t, http.StatusOK))
	if len(rows) != 1 || rows[0].DefaultShiftName == nil || *rows[0].DefaultShiftName != "Malam" {
		t.Errorf("daftar Pejuang: %+v", rows)
	}

	// Masukan tidak valid ditolak dan shift lama tetap.
	const unknown = "00000000-0000-4000-8000-000000000003"
	for _, bad := range []string{"bukan-uuid", unknown} {
		res := sdm.do(http.MethodPatch, path, map[string]any{"default_shift_id": bad}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
		if res.fields()["default_shift_id"] == nil {
			t.Errorf("%s: field tidak dilaporkan: %v", bad, res.fields())
		}
	}
	sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Tes Shift", "email": "tes.shift@pejuang.id", "subdivision_id": subdivisionID(t, "OPS-LAP"),
		"base_salary": "4250000.00", "default_shift_id": unknown,
	}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Shift yang dipakai tidak bisa dihapus lewat API.
	sdm.do(http.MethodDelete, "/v1/admin/shifts/"+shift.ID, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")

	// null melepas shift, lalu shift bisa dihapus.
	got = data[employeeShift](t, sdm.do(http.MethodPatch, path, map[string]any{"default_shift_id": nil}).want(t, http.StatusOK))
	if got.DefaultShiftID != nil || got.DefaultShiftName != nil {
		t.Errorf("null tidak melepas shift: %+v", got)
	}
	sdm.do(http.MethodDelete, "/v1/admin/shifts/"+shift.ID, nil).want(t, http.StatusNoContent)

	// Pembuatan dengan shift langsung tersimpan.
	other := data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Pagi", "start_time": "06:00", "end_time": "14:00", "late_tolerance_minutes": 5, "work_days": []int{1},
	}).want(t, http.StatusCreated))
	withShift := data[employeeShift](t, sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Tes Shift", "email": "tes.shift@pejuang.id", "subdivision_id": subdivisionID(t, "OPS-LAP"),
		"base_salary": "4250000.00", "default_shift_id": other.ID,
	}).want(t, http.StatusCreated))
	if withShift.DefaultShiftName == nil || *withShift.DefaultShiftName != "Pagi" {
		t.Errorf("shift saat dibuat: %+v", withShift)
	}
}

// Kepala subdiv boleh melihat shift anggotanya, tetapi tidak boleh mengubahnya.
func TestEmployeeShiftScope(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	shift := data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Malam", "start_time": "22:00", "end_time": "06:00", "late_tolerance_minutes": 15, "work_days": []int{1},
	}).want(t, http.StatusCreated))
	employeeID := employeeInSubdivision(t, "OPS-GDG")
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+employeeID, map[string]any{"default_shift_id": shift.ID}).want(t, http.StatusOK)

	acc := activeAccount(t, "subdiv", "OPS-GDG")
	head := login(t, srv, acc.email, seedPassword)
	got := data[employeeShift](t, head.do(http.MethodGet, "/v1/admin/employees/"+employeeID, nil).want(t, http.StatusOK))
	if got.DefaultShiftName == nil || *got.DefaultShiftName != "Malam" {
		t.Errorf("kepala subdiv melihat: %+v", got)
	}
	head.do(http.MethodPatch, "/v1/admin/employees/"+employeeID, map[string]any{"default_shift_id": nil}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}

// Mengedit data lain tidak boleh mengganti email login yang berbeda dari email Pejuang.
func TestEmployeeEditKeepsLoginEmail(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	me := data[struct {
		EmployeeID string `json:"employee_id"`
	}](t, sdm.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+me.EmployeeID, map[string]any{"phone": "081234567890"}).want(t, http.StatusOK)
	loginSDM(t, srv)

	// Mengganti email Pejuang tetap memindahkan email login.
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+me.EmployeeID, map[string]any{"email": "sdm.baru@pejuang.id"}).want(t, http.StatusOK)
	login(t, srv, "sdm.baru@pejuang.id", seedPassword)
}
