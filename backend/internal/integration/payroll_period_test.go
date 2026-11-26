package integration

import (
	"net/http"
	"testing"
)

type periodRow struct {
	ID            string  `json:"id"`
	Year          int     `json:"year"`
	Month         int     `json:"month"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Status        string  `json:"status"`
	EmployeeCount int     `json:"employee_count"`
	RejectNote    *string `json:"reject_note"`
	CanFinalize   bool    `json:"can_finalize"`
}

type periodDetail struct {
	Period periodRow `json:"period"`
	Rows   []struct {
		EmployeeID string `json:"employee_id"`
		Gross      string `json:"gross"`
		Deductions string `json:"deductions"`
		Net        string `json:"net"`
		Attendance struct {
			PresentDays int `json:"present_days"`
			LateCount   int `json:"late_count"`
		} `json:"attendance"`
	} `json:"rows"`
	Readiness *struct {
		Review int `json:"review"`
	} `json:"readiness"`
}

type employeePayrollResp struct {
	Row struct {
		Gross      string `json:"gross"`
		Deductions string `json:"deductions"`
		Net        string `json:"net"`
	} `json:"row"`
	Items []struct {
		Code   string `json:"code"`
		Amount string `json:"amount"`
		Manual bool   `json:"manual"`
	} `json:"items"`
	Inputs []struct {
		ComponentID string `json:"component_id"`
		Code        string `json:"code"`
	} `json:"inputs"`
}

// secondSDM menjadikan satu Pejuang seed sebagai SDM kedua (pemeriksa) tanpa kewajiban A2F.
func secondSDM(t *testing.T) string {
	t.Helper()
	a := activeAccount(t, "pejuang", "SDM-PRS")
	exec(t, `UPDATE users SET role = 'sdm', mfa_required = false WHERE id = $1`, a.userID)
	return a.email
}

// TestPayrollPeriodFlow menguji periode ubar (D67): hitung dari presensi dan komponen, isian per periode, syarat
// pengajuan, maker-checker, tolak, kunci presensi, final, dan dibayar.
func TestPayrollPeriodFlow(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	checker := login(t, srv, secondSDM(t), seedPassword)
	kadiv := login(t, srv, activeAccount(t, "kadiv", "OPS-LAP").email, seedPassword)
	employeeID := employeeInSubdivision(t, "OPS-GDG")
	exec(t, `DELETE FROM payroll_components`)
	exec(t, `UPDATE employees SET base_salary = 2600000 WHERE id = $1`, employeeID)

	kadiv.do(http.MethodGet, "/v1/admin/payroll/periods", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	comp := func(body map[string]any) string {
		return data[payrollComponentRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/components", body).want(t, http.StatusCreated)).ID
	}
	comp(map[string]any{"code": "UANG_MAKAN", "name": "Uang makan", "kind": "earning", "method": "per_present_day", "rate": "10000"})
	comp(map[string]any{"code": "POT_TELAT", "name": "Potongan telat", "kind": "deduction", "method": "per_late", "rate": "5000"})
	comp(map[string]any{"code": "POT_ALPHA", "name": "Potongan alpha", "kind": "deduction", "method": "per_absent_day"})
	lembur := comp(map[string]any{"code": "LEMBUR", "name": "Lembur", "kind": "earning", "method": "per_unit", "rate": "20000"})
	pinjaman := comp(map[string]any{"code": "PINJAMAN", "name": "Pinjaman", "kind": "deduction", "method": "manual"})

	// Periode Januari 2020 (cutoff 25): 26 Des 2019 sampai 25 Jan 2020.
	for _, row := range []struct{ date, status string }{
		{"2019-12-30", "present"}, {"2020-01-06", "present"}, {"2020-01-07", "late"}, {"2020-01-08", "absent"},
		{"2020-01-09", "review"}, {"2020-01-27", "present"},
	} {
		exec(t, `INSERT INTO attendances (employee_id, work_date, status, late_minutes) VALUES ($1, $2::date, $3::att_status, 0)`, employeeID, row.date, row.status)
	}

	jan := data[periodRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/periods", map[string]any{"year": 2020, "month": 1}).want(t, http.StatusCreated))
	if jan.StartDate != "2019-12-26" || jan.EndDate != "2020-01-25" || jan.Status != "draft" {
		t.Fatalf("periode = %+v", jan)
	}
	sdm.do(http.MethodPost, "/v1/admin/payroll/periods", map[string]any{"year": 2020, "month": 1}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	base := "/v1/admin/payroll/periods/" + jan.ID

	// Belum dihitung: tidak bisa diajukan.
	sdm.do(http.MethodPost, base+"/submit", nil).wantError(t, http.StatusUnprocessableEntity, "PAYROLL_NOT_READY")

	detail := data[periodDetail](t, sdm.do(http.MethodPost, base+"/generate", nil).want(t, http.StatusOK))
	if detail.Period.EmployeeCount == 0 || detail.Readiness == nil || detail.Readiness.Review != 1 {
		t.Fatalf("setelah hitung = %+v", detail.Period)
	}
	// 3 hadir (termasuk telat) x 10.000; telat 1 x 5.000; alpha 2.600.000 / 26 = 100.000.
	emp := data[employeePayrollResp](t, sdm.do(http.MethodGet, base+"/employees/"+employeeID, nil).want(t, http.StatusOK))
	if emp.Row.Gross != "2630000.00" || emp.Row.Deductions != "105000.00" || emp.Row.Net != "2525000.00" || len(emp.Inputs) != 2 {
		t.Fatalf("slip = %+v", emp)
	}

	// Isian per periode: lembur 4 jam dan pinjaman 50.000.
	emp = data[employeePayrollResp](t, sdm.do(http.MethodPut, base+"/employees/"+employeeID, map[string]any{"inputs": []map[string]any{
		{"component_id": lembur, "quantity": "4"}, {"component_id": pinjaman, "amount": "50000", "note": "Cicilan ke-1"},
	}}).want(t, http.StatusOK))
	if emp.Row.Gross != "2710000.00" || emp.Row.Deductions != "155000.00" || emp.Row.Net != "2555000.00" {
		t.Errorf("setelah isian = %+v", emp.Row)
	}
	sdm.do(http.MethodPut, base+"/employees/"+employeeID, map[string]any{"inputs": []map[string]any{{"component_id": lembur, "quantity": "-1"}}}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Masih ada presensi review: pengajuan ditolak.
	r := sdm.do(http.MethodPost, base+"/submit", nil).wantError(t, http.StatusUnprocessableEntity, "PAYROLL_NOT_READY")
	if details, _ := r.errorField("details").(map[string]any); details["review"] != float64(1) {
		t.Errorf("rincian = %v", r.errorField("details"))
	}
	exec(t, `UPDATE attendances SET status = 'present' WHERE employee_id = $1 AND work_date = '2020-01-09'`, employeeID)

	// Hitung ulang mempertahankan isian.
	sdm.do(http.MethodPost, base+"/generate", nil).want(t, http.StatusOK)
	emp = data[employeePayrollResp](t, sdm.do(http.MethodGet, base+"/employees/"+employeeID, nil).want(t, http.StatusOK))
	if emp.Row.Gross != "2720000.00" || emp.Row.Net != "2565000.00" {
		t.Errorf("hitung ulang = %+v", emp.Row)
	}

	detail = data[periodDetail](t, sdm.do(http.MethodPost, base+"/submit", nil).want(t, http.StatusOK))
	if detail.Period.Status != "pending_approval" || detail.Period.CanFinalize {
		t.Errorf("setelah diajukan = %+v", detail.Period)
	}
	sdm.do(http.MethodPost, base+"/generate", nil).wantError(t, http.StatusConflict, "PERIOD_LOCKED")
	sdm.do(http.MethodPut, base+"/employees/"+employeeID, map[string]any{"inputs": []map[string]any{}}).wantError(t, http.StatusConflict, "PERIOD_LOCKED")

	// Presensi di periode yang diajukan terkunci.
	var locked string
	if err := ownerPool.QueryRow(t.Context(), `INSERT INTO attendances (employee_id, work_date, status, late_minutes)
		SELECT id, '2020-01-10', 'review', 0 FROM employees WHERE id <> $1 AND status = 'active' LIMIT 1 RETURNING id::text`, employeeID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	checker.do(http.MethodPost, "/v1/admin/attendance/"+locked+"/resolve", map[string]any{"decision": "present"}).wantError(t, http.StatusConflict, "PERIOD_LOCKED")
	exec(t, `DELETE FROM attendances WHERE id = $1`, locked)

	// Maker-checker: yang mengajukan tidak bisa memfinalkan.
	sdm.do(http.MethodPost, base+"/finalize", nil).wantError(t, http.StatusForbidden, "MAKER_CHECKER_VIOLATION")
	checker.do(http.MethodPost, base+"/reject", map[string]any{"note": "x"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	detail = data[periodDetail](t, checker.do(http.MethodPost, base+"/reject", map[string]any{"note": "Lembur belum lengkap"}).want(t, http.StatusOK))
	if detail.Period.Status != "draft" || detail.Period.RejectNote == nil || *detail.Period.RejectNote != "Lembur belum lengkap" {
		t.Errorf("setelah ditolak = %+v", detail.Period)
	}

	sdm.do(http.MethodPost, base+"/submit", nil).want(t, http.StatusOK)
	sdm.do(http.MethodPost, base+"/mark-paid", nil).wantError(t, http.StatusConflict, "PERIOD_LOCKED")
	if p := data[periodDetail](t, checker.do(http.MethodGet, base, nil).want(t, http.StatusOK)).Period; !p.CanFinalize {
		t.Errorf("pemeriksa harus bisa memfinalkan: %+v", p)
	}
	detail = data[periodDetail](t, checker.do(http.MethodPost, base+"/finalize", nil).want(t, http.StatusOK))
	if detail.Period.Status != "final" {
		t.Errorf("final = %+v", detail.Period)
	}
	var published int
	if err := ownerPool.QueryRow(t.Context(), `SELECT count(*) FROM payrolls WHERE period_id = $1 AND published_at IS NULL`, jan.ID).Scan(&published); err != nil || published != 0 {
		t.Errorf("slip belum terbit: %d, %v", published, err)
	}
	detail = data[periodDetail](t, sdm.do(http.MethodPost, base+"/mark-paid", nil).want(t, http.StatusOK))
	if detail.Period.Status != "paid" {
		t.Errorf("dibayar = %+v", detail.Period)
	}
	sdm.do(http.MethodDelete, base, nil).wantError(t, http.StatusConflict, "PERIOD_LOCKED")

	// Periode berikutnya bersambung, dan presensi Februari tidak ikut terkunci.
	feb := data[periodRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/periods", map[string]any{"year": 2020, "month": 2}).want(t, http.StatusCreated))
	if feb.StartDate != "2020-01-26" || feb.EndDate != "2020-02-25" {
		t.Errorf("Februari = %+v", feb)
	}
	sdm.do(http.MethodPost, "/v1/admin/payroll/periods", map[string]any{"year": 2019, "month": 12}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodDelete, "/v1/admin/payroll/periods/"+feb.ID, nil).want(t, http.StatusNoContent)

	list := data[[]periodRow](t, sdm.do(http.MethodGet, "/v1/admin/payroll/periods", nil).want(t, http.StatusOK))
	if len(list) != 1 || list[0].Status != "paid" {
		t.Errorf("daftar = %+v", list)
	}
	for action, want := range map[string]int{
		"payroll.period_created": 3, "payroll.generated": 2, "payroll.adjusted": 1, "payroll.submitted": 2,
		"payroll.rejected": 1, "payroll.finalized": 1, "payroll.paid": 1,
	} {
		if n := countAudit(t, action); n != want {
			t.Errorf("audit %s = %d, ingin %d", action, n, want)
		}
	}
}
