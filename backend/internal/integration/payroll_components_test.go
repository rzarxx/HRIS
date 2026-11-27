package integration

import (
	"net/http"
	"testing"
)

type payrollComponentRow struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Method     string  `json:"method"`
	Rate       *string `json:"rate"`
	Percent    *string `json:"percent"`
	Divisor    *string `json:"divisor"`
	Active     bool    `json:"active"`
	ShowOnSlip bool    `json:"show_on_slip"`
	SortOrder  int     `json:"sort_order"`
}

type payrollSettingsRow struct {
	PeriodCutoffDay int     `json:"period_cutoff_day"`
	MinNetZero      bool    `json:"min_net_zero"`
	UpdatedAt       *string `json:"updated_at"`
}

type employeePayrollComponentRow struct {
	ComponentID string  `json:"component_id"`
	Code        string  `json:"code"`
	Amount      *string `json:"amount"`
}

// TestPayrollComponents menguji komponen ubar (D66): hanya SDM, validasi per cara hitung, kode tetap,
// nonaktif tanpa hapus, setelan cutoff, dan nilai komponen per Pejuang.
func TestPayrollComponents(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	kadiv := login(t, srv, activeAccount(t, "kadiv", "OPS-LAP").email, seedPassword)
	super, _ := superadminClient(t, srv, "IT-DEV")
	pejuang := activeAccount(t, "pejuang", "OPS-GDG")

	for _, c := range []*client{kadiv, super} {
		c.do(http.MethodGet, "/v1/admin/payroll/components", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
		c.do(http.MethodGet, "/v1/admin/payroll/settings", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
		c.do(http.MethodGet, "/v1/admin/employees/"+pejuang.employeeID+"/payroll-components", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	}

	// Seed membuat contoh komponen nonaktif; dibersihkan agar tes mulai dari kosong.
	seeded := data[[]payrollComponentRow](t, sdm.do(http.MethodGet, "/v1/admin/payroll/components", nil).want(t, http.StatusOK))
	if len(seeded) != 7 {
		t.Fatalf("contoh seed = %d, ingin 7", len(seeded))
	}
	for _, c := range seeded {
		if c.Active {
			t.Errorf("contoh seed %s harus nonaktif", c.Code)
		}
	}
	exec(t, `DELETE FROM payroll_components`)

	// Validasi: cara hitung harian butuh nominal, persen butuh persen, kode UBAR_POKOK dipakai sistem.
	r := sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "UANG_MAKAN", "name": "Uang makan", "kind": "earning", "method": "per_present_day"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	if r.fields()["rate"] == nil {
		t.Errorf("rate kosong harus ditolak: %v", r.fields())
	}
	r = sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "9pph", "name": "P", "kind": "x", "method": "percent_of_gross"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	for _, f := range []string{"code", "name", "kind", "percent"} {
		if r.fields()[f] == nil {
			t.Errorf("field %s harus ditolak: %v", f, r.fields())
		}
	}
	sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "UBAR_POKOK", "name": "Pokok", "kind": "earning", "method": "manual"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	makan := data[payrollComponentRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{
		"code": "uang_makan", "name": " Uang  makan ", "kind": "earning", "method": "per_present_day", "rate": "15000", "percent": "5", "sort_order": 10,
	}).want(t, http.StatusCreated))
	if makan.Code != "UANG_MAKAN" || makan.Name != "Uang makan" || makan.Rate == nil || *makan.Rate != "15000.00" || makan.Percent != nil || !makan.Active || !makan.ShowOnSlip {
		t.Fatalf("dibuat = %+v", makan)
	}
	sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "UANG_MAKAN", "name": "Lagi", "kind": "earning", "method": "manual"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	pph := data[payrollComponentRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{
		"code": "PPH21", "name": "PPh 21", "kind": "deduction", "method": "percent_of_gross", "percent": "2.5",
	}).want(t, http.StatusCreated))
	if pph.Percent == nil || *pph.Percent != "2.500" || pph.Rate != nil {
		t.Errorf("pph = %+v", pph)
	}
	jabatan := data[payrollComponentRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{
		"code": "TUNJ_JABATAN", "name": "Tunjangan jabatan", "kind": "earning", "method": "per_employee", "rate": "999",
	}).want(t, http.StatusCreated))
	if jabatan.Rate != nil {
		t.Errorf("per_employee tidak memakai rate: %+v", jabatan)
	}

	// PATCH: field lain tetap, kode tidak bisa diubah, nonaktif lewat active=false.
	sdm.do(http.MethodPatch, "/v1/admin/payroll/components/"+makan.ID, map[string]any{"code": "MAKAN"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	off := data[payrollComponentRow](t, sdm.do(http.MethodPatch, "/v1/admin/payroll/components/"+makan.ID, map[string]any{"active": false, "show_on_slip": false}).
		want(t, http.StatusOK))
	if off.Active || off.ShowOnSlip || off.Rate == nil || *off.Rate != "15000.00" || off.SortOrder != 10 || off.Name != "Uang makan" {
		t.Errorf("setelah nonaktif = %+v", off)
	}
	sdm.do(http.MethodPatch, "/v1/admin/payroll/components/00000000-0000-0000-0000-000000000001", map[string]any{"active": false}).
		wantError(t, http.StatusNotFound, "NOT_FOUND")
	if rows := data[[]payrollComponentRow](t, sdm.do(http.MethodGet, "/v1/admin/payroll/components", nil).want(t, http.StatusOK)); len(rows) != 3 || rows[0].Code != "PPH21" {
		t.Errorf("daftar = %+v", rows)
	}

	// Setelan payroll: bawaan, validasi, simpan.
	def := data[payrollSettingsRow](t, sdm.do(http.MethodGet, "/v1/admin/payroll/settings", nil).want(t, http.StatusOK))
	if def.PeriodCutoffDay != 25 || !def.MinNetZero || def.UpdatedAt != nil {
		t.Errorf("setelan bawaan = %+v", def)
	}
	sdm.do(http.MethodPut, "/v1/admin/payroll/settings", map[string]any{"period_cutoff_day": 31}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	set := data[payrollSettingsRow](t, sdm.do(http.MethodPut, "/v1/admin/payroll/settings", map[string]any{"period_cutoff_day": 0, "min_net_zero": false}).
		want(t, http.StatusOK))
	if set.PeriodCutoffDay != 0 || set.MinNetZero || set.UpdatedAt == nil {
		t.Errorf("setelan disimpan = %+v", set)
	}

	// Nilai per Pejuang: hanya komponen per_employee.
	path := "/v1/admin/employees/" + pejuang.employeeID + "/payroll-components"
	list := data[[]employeePayrollComponentRow](t, sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK))
	if len(list) != 1 || list[0].Code != "TUNJ_JABATAN" || list[0].Amount != nil {
		t.Fatalf("awal per Pejuang = %+v", list)
	}
	sdm.do(http.MethodPut, path, map[string]any{"components": []map[string]any{{"component_id": pph.ID, "amount": "1"}}}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPut, path, map[string]any{"components": []map[string]any{{"component_id": jabatan.ID, "amount": "-5"}}}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	saved := data[[]employeePayrollComponentRow](t, sdm.do(http.MethodPut, path, map[string]any{
		"components": []map[string]any{{"component_id": jabatan.ID, "amount": "750000"}},
	}).want(t, http.StatusOK))
	if len(saved) != 1 || saved[0].Amount == nil || *saved[0].Amount != "750000.00" {
		t.Errorf("tersimpan = %+v", saved)
	}
	cleared := data[[]employeePayrollComponentRow](t, sdm.do(http.MethodPut, path, map[string]any{"components": []map[string]any{}}).want(t, http.StatusOK))
	if cleared[0].Amount != nil {
		t.Errorf("dilepas = %+v", cleared)
	}
	sdm.do(http.MethodGet, "/v1/admin/employees/00000000-0000-0000-0000-000000000001/payroll-components", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")

	// 3 tambah + 1 ubah + 1 setelan + 2 simpan per Pejuang.
	if n := countAudit(t, "payroll.rules_changed"); n != 7 {
		t.Errorf("audit payroll.rules_changed = %d, ingin 7", n)
	}
}
