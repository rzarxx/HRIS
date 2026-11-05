package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func jsonUnmarshal(raw []byte, dst any) error { return json.Unmarshal(raw, dst) }

type summary struct {
	EmployeeCount    int    `json:"employee_count"`
	DivisionCount    int    `json:"division_count"`
	SubdivisionCount int    `json:"subdivision_count"`
	PresentToday     int    `json:"present_today"`
	WorkDate         string `json:"work_date"`
}

type orgChart struct {
	EmployeesPerSubdivision []struct {
		Division      string `json:"division"`
		Subdivision   string `json:"subdivision"`
		EmployeeCount int    `json:"employee_count"`
	} `json:"employees_per_subdivision"`
	SubdivisionsPerDivision []struct {
		Division         string `json:"division"`
		SubdivisionCount int    `json:"subdivision_count"`
	} `json:"subdivisions_per_division"`
}

func countIn(chart orgChart, subdivision string) int {
	for _, s := range chart.EmployeesPerSubdivision {
		if s.Subdivision == subdivision {
			return s.EmployeeCount
		}
	}
	return -1
}

// Inti demo: menambah Pejuang langsung menggeser grafik (F23).
func TestDashboardFollowsEmployees(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	before := data[summary](t, sdm.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).want(t, http.StatusOK))
	if before.EmployeeCount != 57 || before.DivisionCount != 4 || before.SubdivisionCount != 11 {
		t.Errorf("ringkasan seed: %+v", before)
	}
	if before.WorkDate != time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02") {
		t.Errorf("work_date = %s, harus tanggal WIB", before.WorkDate)
	}
	chart := data[orgChart](t, sdm.do(http.MethodGet, "/v1/admin/dashboard/org-chart", nil).want(t, http.StatusOK))
	if got := countIn(chart, "Lapangan"); got != 12 {
		t.Errorf("Lapangan = %d, ingin 12", got)
	}
	if len(chart.SubdivisionsPerDivision) != 4 {
		t.Errorf("divisi di grafik = %d", len(chart.SubdivisionsPerDivision))
	}

	created := createEmployee(t, sdm, "tambah.demo@pejuang.id")
	after := data[summary](t, sdm.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).want(t, http.StatusOK))
	if after.EmployeeCount != 58 {
		t.Errorf("setelah tambah = %d", after.EmployeeCount)
	}
	chart = data[orgChart](t, sdm.do(http.MethodGet, "/v1/admin/dashboard/org-chart", nil).want(t, http.StatusOK))
	if got := countIn(chart, "Lapangan"); got != 13 {
		t.Errorf("Lapangan setelah tambah = %d", got)
	}

	sdm.do(http.MethodPost, "/v1/admin/employees/"+created.ID+"/deactivate", nil).want(t, http.StatusOK)
	chart = data[orgChart](t, sdm.do(http.MethodGet, "/v1/admin/dashboard/org-chart", nil).want(t, http.StatusOK))
	if got := countIn(chart, "Lapangan"); got != 12 {
		t.Errorf("Lapangan setelah nonaktif = %d", got)
	}
}

func TestEmployeeCRUD(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	if f := sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{"full_name": "Al"}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED").fields(); f["full_name"] == nil || f["email"] == nil || f["subdivision_id"] == nil || f["base_salary"] == nil {
		t.Errorf("pesan per field: %v", f)
	}
	sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{"full_name": "Budi", "field_ngawur": 1}).want(t, http.StatusBadRequest)
	sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{"full_name": "Budi Santoso", "email": "b@pejuang.id", "subdivision_id": "00000000-0000-4000-8000-000000000000", "base_salary": "1.00"}).
		want(t, http.StatusBadRequest)

	created := createEmployee(t, sdm, "crud.satu@pejuang.id")
	if created.BaseSalary != "4250000.00" {
		t.Errorf("uang harus string 2 digit: %q", created.BaseSalary)
	}

	// Email dan NIPY unik, termasuk beda huruf besar-kecil.
	dup := sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Rudi Lain", "email": "CRUD.Satu@pejuang.id", "subdivision_id": subdivisionID(t, "OPS-LAP"), "base_salary": "1.00",
	}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	if dup.fields()["email"] == nil {
		t.Errorf("duplikat email: %s", dup.body)
	}
	dup = sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Rudi Lain", "email": "crud.dua@pejuang.id", "nip": created.Nip, "subdivision_id": subdivisionID(t, "OPS-LAP"), "base_salary": "1.00",
	}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	if dup.fields()["nip"] == nil {
		t.Errorf("duplikat NIPY: %s", dup.body)
	}
	// Transaksi gagal tidak meninggalkan data setengah jadi.
	var orphan int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM employees WHERE email = 'crud.dua@pejuang.id'`).Scan(&orphan); err != nil {
		t.Fatal(err)
	}
	if orphan != 0 {
		t.Error("data tersisa setelah gagal")
	}

	// PATCH hanya mengubah field yang dikirim.
	patched := data[struct {
		FullName   string  `json:"full_name"`
		Nip        string  `json:"nip"`
		Phone      *string `json:"phone"`
		BaseSalary string  `json:"base_salary"`
	}](t, sdm.do(http.MethodPatch, "/v1/admin/employees/"+created.ID, map[string]any{"phone": "081234567890"}).want(t, http.StatusOK))
	if patched.FullName != "Rudi Hartono" || patched.Nip != created.Nip || patched.BaseSalary != "4250000.00" || patched.Phone == nil {
		t.Errorf("PATCH: %+v", patched)
	}

	// Ubah gaji tercatat dengan nilai lama dan baru.
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+created.ID, map[string]any{"base_salary": "5000000.00"}).want(t, http.StatusOK)
	var detail string
	if err := ownerPool.QueryRow(context.Background(), `SELECT detail::text FROM audit_logs WHERE action = 'employee.salary_changed'`).Scan(&detail); err != nil {
		t.Fatalf("audit ubah gaji: %v", err)
	}
	if detail == "" {
		t.Error("detail audit kosong")
	}

	// Ganti email ikut mengganti email login.
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+created.ID, map[string]any{"email": "crud.baru@pejuang.id"}).want(t, http.StatusOK)
	var login string
	if err := ownerPool.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, created.Account.UserID).Scan(&login); err != nil {
		t.Fatal(err)
	}
	if login != "crud.baru@pejuang.id" {
		t.Errorf("email login = %s", login)
	}

	sdm.do(http.MethodGet, "/v1/admin/employees/bukan-uuid", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodGet, "/v1/admin/employees/00000000-0000-4000-8000-000000000000", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestEmployeeListFilters(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	get := func(query string) ([]employeeRow, int) {
		res := sdm.do(http.MethodGet, "/v1/admin/employees?"+query, nil).want(t, http.StatusOK)
		var parsed struct {
			Data []employeeRow `json:"data"`
			Meta employeeMeta  `json:"meta"`
		}
		if err := json.Unmarshal(res.body, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed.Data, parsed.Meta.Total
	}
	rows, total := get("")
	if len(rows) != 20 || total != 58 {
		t.Errorf("default per_page: %d dari %d", len(rows), total)
	}
	if rows[0].Account == nil || rows[0].BaseSalary == nil {
		t.Error("SDM harus melihat gaji dan data akun")
	}
	rows, _ = get("per_page=7&page=9")
	if len(rows) != 2 {
		t.Errorf("halaman terakhir = %d", len(rows))
	}
	_, inactive := get("status=inactive")
	if inactive != 1 {
		t.Errorf("nonaktif = %d", inactive)
	}
	_, lapangan := get("subdivision_id=" + subdivisionID(t, "OPS-LAP"))
	if lapangan != 12 {
		t.Errorf("Lapangan = %d", lapangan)
	}
	_, none := get("q=tidak-ada-yang-seperti-ini")
	if none != 0 {
		t.Errorf("pencarian kosong = %d", none)
	}
	// Karakter khusus tetap aman karena query berparameter.
	_, injection := get("q=%27%3B%20DROP%20TABLE%20employees%3B--")
	if injection != 0 {
		t.Errorf("pencarian injeksi = %d", injection)
	}
	sdm.do(http.MethodGet, "/v1/admin/employees?per_page=101", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodGet, "/v1/admin/employees?status=dipecat", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestOrganizationCRUD(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	division := data[struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/divisions", map[string]any{"code": "hkm", "name": "Hukum"}).want(t, http.StatusCreated))
	if division.Code != "HKM" {
		t.Errorf("kode harus huruf besar: %s", division.Code)
	}
	sdm.do(http.MethodPost, "/v1/admin/divisions", map[string]any{"code": "HKM", "name": "Hukum Lain"}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	sdm.do(http.MethodPost, "/v1/admin/divisions", map[string]any{"code": "A B", "name": ""}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	patched := data[struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}](t, sdm.do(http.MethodPatch, "/v1/admin/divisions/"+division.ID, map[string]any{"name": "Legal"}).want(t, http.StatusOK))
	if patched.Code != "HKM" || patched.Name != "Legal" {
		t.Errorf("PATCH divisi: %+v", patched)
	}

	sub := data[struct {
		ID string `json:"id"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/subdivisions", map[string]any{"division_id": division.ID, "code": "HKM-KTR", "name": "Kontrak"}).want(t, http.StatusCreated))
	sdm.do(http.MethodDelete, "/v1/admin/divisions/"+division.ID, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")

	// Kepala harus anggota unit itu.
	outsider := employeeInSubdivision(t, "OPS-GDG")
	sdm.do(http.MethodPatch, "/v1/admin/subdivisions/"+sub.ID, map[string]any{"head_employee_id": outsider}).want(t, http.StatusBadRequest)
	sdm.do(http.MethodPatch, "/v1/admin/divisions/"+division.ID, map[string]any{"head_employee_id": outsider}).want(t, http.StatusBadRequest)

	member := data[struct {
		ID string `json:"id"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Anggota Kontrak", "email": "kontrak@pejuang.id", "subdivision_id": sub.ID, "base_salary": "1000000.00",
	}).want(t, http.StatusCreated))
	sdm.do(http.MethodPatch, "/v1/admin/subdivisions/"+sub.ID, map[string]any{"head_employee_id": member.ID}).want(t, http.StatusOK)

	// Pejuang nonaktif tetap dihitung agar riwayatnya tidak kehilangan rujukan.
	sdm.do(http.MethodDelete, "/v1/admin/subdivisions/"+sub.ID, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")
	sdm.do(http.MethodPost, "/v1/admin/employees/"+member.ID+"/deactivate", nil).want(t, http.StatusOK)
	sdm.do(http.MethodDelete, "/v1/admin/subdivisions/"+sub.ID, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")

	empty := data[struct {
		ID string `json:"id"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/divisions", map[string]any{"code": "TMP", "name": "Sementara"}).want(t, http.StatusCreated))
	sdm.do(http.MethodDelete, "/v1/admin/divisions/"+empty.ID, nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodDelete, "/v1/admin/divisions/"+empty.ID, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestAnnouncementCRUD(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	type announcement struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		Body         string  `json:"body"`
		Type         string  `json:"type"`
		DivisionName *string `json:"division_name"`
		IsActive     bool    `json:"is_active"`
	}
	starts := "2026-11-02T01:00:00Z"
	created := data[announcement](t, sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "Rapat", "body": "<script>alert(1)</script> Halo\x07", "type": "popup", "starts_at": starts,
	}).want(t, http.StatusCreated))
	if !created.IsActive || created.Type != "popup" || created.DivisionName != nil {
		t.Errorf("baru: %+v", created)
	}
	// Disimpan sebagai teks; karakter kontrol dibuang, HTML tidak pernah dirender klien.
	if created.Body != "<script>alert(1)</script> Halo" {
		t.Errorf("isi: %q", created.Body)
	}

	if f := sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "", "body": "", "type": "toast", "starts_at": "kemarin",
	}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED").fields(); f["title"] == nil || f["body"] == nil || f["type"] == nil || f["starts_at"] == nil {
		t.Errorf("validasi: %v", f)
	}
	sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "A", "body": "B", "type": "banner", "starts_at": starts, "ends_at": "2026-11-01T00:00:00Z",
	}).want(t, http.StatusBadRequest)

	patched := data[announcement](t, sdm.do(http.MethodPatch, "/v1/admin/announcements/"+created.ID, map[string]any{"is_active": false}).want(t, http.StatusOK))
	if patched.IsActive || patched.Title != "Rapat" {
		t.Errorf("PATCH: %+v", patched)
	}
	sdm.do(http.MethodDelete, "/v1/admin/announcements/"+created.ID, nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodGet, "/v1/admin/announcements/"+created.ID, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	if countAudit(t, "announcement.created") != 1 || countAudit(t, "announcement.updated") != 1 || countAudit(t, "announcement.deleted") != 1 {
		t.Error("aksi pengumuman harus tercatat")
	}
}

func TestPayrollPreviewUsesAttendance(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	employeeID := employeeInSubdivision(t, "OPS-GDG")
	exec(t, `UPDATE employees SET base_salary = 5200000 WHERE id = $1`, employeeID)
	// Periode lama yang tidak tersentuh simulasi presensi.
	for _, row := range []struct {
		date, status string
		late         int
	}{
		{"2020-01-06", "present", 0},
		{"2020-01-07", "late", 25},
		{"2020-01-08", "late", 15},
		{"2020-01-09", "absent", 0},
		{"2020-01-10", "leave", 0},
	} {
		exec(t, `INSERT INTO attendances (employee_id, work_date, status, late_minutes) VALUES ($1, $2::date, $3::att_status, $4)`, employeeID, row.date, row.status, row.late)
	}

	report := data[[]struct {
		EmployeeID  string `json:"employee_id"`
		WorkDays    int    `json:"work_days"`
		Present     int    `json:"present"`
		Late        int    `json:"late"`
		Leave       int    `json:"leave"`
		Absent      int    `json:"absent"`
		LateMinutes int    `json:"late_minutes"`
	}](t, sdm.do(http.MethodGet, "/v1/admin/reports/attendance?year=2020&month=1", nil).want(t, http.StatusOK))
	found := false
	for _, r := range report {
		if r.EmployeeID != employeeID {
			continue
		}
		found = true
		if r.WorkDays != 5 || r.Present != 3 || r.Late != 2 || r.Leave != 1 || r.Absent != 1 || r.LateMinutes != 40 {
			t.Errorf("rekap: %+v", r)
		}
	}
	if !found {
		t.Fatal("Pejuang tidak ada di rekap")
	}

	res := sdm.do(http.MethodGet, "/v1/admin/reports/payroll-preview?year=2020&month=1", nil).want(t, http.StatusOK)
	var parsed struct {
		Data []struct {
			EmployeeID      string `json:"employee_id"`
			BaseSalary      string `json:"base_salary"`
			AbsentDeduction string `json:"absent_deduction"`
			LateDeduction   string `json:"late_deduction"`
			Net             string `json:"net"`
		} `json:"data"`
		Meta struct {
			Rules struct {
				Version string `json:"version"`
				Note    string `json:"note"`
			} `json:"rules"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(res.body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Meta.Rules.Note == "" || parsed.Meta.Rules.Version == "" {
		t.Error("rumus sementara harus ikut dikirim")
	}
	for _, r := range parsed.Data {
		if r.EmployeeID != employeeID {
			continue
		}
		if r.BaseSalary != "5200000.00" || r.AbsentDeduction != "200000.00" || r.LateDeduction != "10000.00" || r.Net != "4990000.00" {
			t.Errorf("upah: %+v", r)
		}
	}

	sdm.do(http.MethodGet, "/v1/admin/reports/attendance?year=2020&month=13", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodGet, "/v1/admin/reports/attendance", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestAttendanceReportWorkMode(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	id := employeeInSubdivision(t, "OPS-GDG")
	for _, row := range []struct{ date, mode, status string }{
		{"2020-02-03", "onsite", "present"},
		{"2020-02-04", "wfa", "late"},
		{"2020-02-05", "field", "present"},
	} {
		exec(t, `INSERT INTO attendances (employee_id, work_date, status, work_mode, late_minutes, clock_in_at) VALUES ($1, $2::date, $3::att_status, $4, 10, $2::date::timestamptz)`, id, row.date, row.status, row.mode)
	}
	exec(t, `INSERT INTO attendances (employee_id, work_date, status, work_mode) VALUES ($1, '2020-02-06', 'absent', 'onsite')`, id)
	type row struct {
		EmployeeID  string `json:"employee_id"`
		Present     int    `json:"present"`
		Late        int    `json:"late"`
		LateMinutes int    `json:"late_minutes"`
	}
	for _, tc := range []struct {
		mode                   string
		present, late, minutes int
	}{
		{"", 3, 1, 30}, {"onsite", 1, 0, 10}, {"wfa", 1, 1, 10}, {"field", 1, 0, 10},
	} {
		rows := data[[]row](t, sdm.do(http.MethodGet, "/v1/admin/reports/attendance?year=2020&month=2&work_mode="+tc.mode, nil).want(t, http.StatusOK))
		found := false
		for _, r := range rows {
			if r.EmployeeID == id {
				found = true
				if r.Present != tc.present || r.Late != tc.late || r.LateMinutes != tc.minutes {
					t.Errorf("mode %q: %+v", tc.mode, r)
				}
			} else if tc.mode != "" {
				t.Errorf("mode %q memuat Pejuang tanpa catatan: %+v", tc.mode, r)
			}
		}
		if !found {
			t.Errorf("mode %q: Pejuang tidak ditemukan", tc.mode)
		}
	}
	sdm.do(http.MethodGet, "/v1/admin/reports/attendance?year=2020&month=2&work_mode=invalid", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

// Pejuang baru tidak muncul di laporan bulan sebelum bergabung, dan yang nonaktif tanpa
// presensi tidak muncul dengan gaji penuh.
func TestReportMembership(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	created := data[struct {
		ID string `json:"id"`
	}](t, sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Pejuang Baru", "email": "pejuang.baru@pejuang.id", "subdivision_id": subdivisionID(t, "OPS-LAP"),
		"base_salary": "1000000.00", "join_date": "2026-11-15",
	}).want(t, http.StatusCreated))

	rows := func(year, month string) map[string]bool {
		list := data[[]struct {
			EmployeeID string `json:"employee_id"`
		}](t, sdm.do(http.MethodGet, "/v1/admin/reports/payroll-preview?year="+year+"&month="+month, nil).want(t, http.StatusOK))
		out := map[string]bool{}
		for _, r := range list {
			out[r.EmployeeID] = true
		}
		return out
	}
	if rows("2026", "10")[created.ID] {
		t.Error("muncul di bulan sebelum bergabung")
	}
	if !rows("2026", "12")[created.ID] {
		t.Error("harus muncul setelah bergabung")
	}
	var inactive string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM employees WHERE status = 'inactive' LIMIT 1`).Scan(&inactive); err != nil {
		t.Fatal(err)
	}
	if rows("2026", "12")[inactive] {
		t.Error("Pejuang nonaktif tanpa presensi tidak boleh muncul")
	}
}

func TestReadiness(t *testing.T) {
	srv := fresh(t, options{})
	c := newClient(t, srv)
	c.do(http.MethodGet, "/healthz", nil).want(t, http.StatusOK)
	c.do(http.MethodGet, "/readyz", nil).want(t, http.StatusOK)
	c.do(http.MethodGet, "/v1/tidak-ada", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}
