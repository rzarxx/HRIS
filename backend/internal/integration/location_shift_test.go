package integration

import (
	"net/http"
	"testing"
)

type officeLocation struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusM   int     `json:"radius_m"`
	Polygon   []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"polygon"`
	IsActive      bool `json:"is_active"`
	EmployeeCount int  `json:"employee_count"`
}

type shiftRow struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	StartTime            string `json:"start_time"`
	EndTime              string `json:"end_time"`
	LateToleranceMinutes int    `json:"late_tolerance_minutes"`
	WorkDays             []int  `json:"work_days"`
}

func validLocation() map[string]any {
	return map[string]any{"name": "Kantor Pusat", "latitude": -6.2001, "longitude": 106.8166, "radius_m": 100}
}

func TestOfficeLocationLifecycle(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	created := data[officeLocation](t, sdm.do(http.MethodPost, "/v1/admin/office-locations", validLocation()).want(t, http.StatusCreated))
	if created.Latitude != -6.2001 || created.Longitude != 106.8166 || created.RadiusM != 100 || !created.IsActive || created.Polygon != nil {
		t.Errorf("lokasi baru: %+v", created)
	}
	if got := countAudit(t, "settings.changed"); got != 1 {
		t.Errorf("audit settings.changed = %d", got)
	}

	// PATCH sebagian: field lain tetap, poligon bisa dipasang lalu dilepas dengan null.
	path := "/v1/admin/office-locations/" + created.ID
	polygon := []map[string]float64{{"latitude": 0, "longitude": 0}, {"latitude": 0, "longitude": 1}, {"latitude": 1, "longitude": 1}}
	updated := data[officeLocation](t, sdm.do(http.MethodPatch, path, map[string]any{"polygon": polygon}).want(t, http.StatusOK))
	if len(updated.Polygon) != 3 || updated.Name != "Kantor Pusat" || updated.RadiusM != 100 {
		t.Errorf("PATCH poligon: %+v", updated)
	}
	renamed := data[officeLocation](t, sdm.do(http.MethodPatch, path, map[string]any{"name": "Kantor Baru"}).want(t, http.StatusOK))
	if renamed.Name != "Kantor Baru" || len(renamed.Polygon) != 3 {
		t.Errorf("PATCH nama menghilangkan poligon: %+v", renamed)
	}
	cleared := data[officeLocation](t, sdm.do(http.MethodPatch, path, map[string]any{"polygon": nil}).want(t, http.StatusOK))
	if cleared.Polygon != nil {
		t.Errorf("poligon tidak terlepas: %+v", cleared.Polygon)
	}

	sdm.do(http.MethodDelete, path, nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodDelete, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodPatch, path, map[string]any{"name": "x"}).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestOfficeLocationValidation(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	three := []map[string]float64{{"latitude": 0, "longitude": 0}, {"latitude": 0, "longitude": 1}, {"latitude": 1, "longitude": 1}}
	two := three[:2]
	cases := []struct {
		name, field string
		key         string
		value       any
	}{
		{"nama kosong", "name", "name", "  "},
		{"lintang di luar rentang", "latitude", "latitude", 91},
		{"lintang tidak dikirim", "latitude", "latitude", nil},
		{"bujur di luar rentang", "longitude", "longitude", -181},
		{"radius terlalu kecil", "radius_m", "radius_m", 9},
		{"radius terlalu besar", "radius_m", "radius_m", 5001},
		{"poligon dua titik", "polygon", "polygon", two},
		{"poligon kosong", "polygon", "polygon", []map[string]float64{}},
		{"titik poligon rusak", "polygon", "polygon", append([]map[string]float64{{"latitude": 95, "longitude": 0}}, two...)},
	}
	for _, c := range cases {
		body := validLocation()
		body[c.key] = c.value
		res := sdm.do(http.MethodPost, "/v1/admin/office-locations", body).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
		if res.fields()[c.field] == nil {
			t.Errorf("%s: field %s tidak dilaporkan: %v", c.name, c.field, res.fields())
		}
	}
	if rows := data[[]officeLocation](t, sdm.do(http.MethodGet, "/v1/admin/office-locations", nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Errorf("input tidak valid tetap tersimpan: %+v", rows)
	}
}

func TestEmployeeLocations(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	employeeID := employeeInSubdivision(t, "OPS-LAP")
	a := data[officeLocation](t, sdm.do(http.MethodPost, "/v1/admin/office-locations", validLocation()).want(t, http.StatusCreated))
	other := validLocation()
	other["name"] = "Cabang Bandung"
	b := data[officeLocation](t, sdm.do(http.MethodPost, "/v1/admin/office-locations", other).want(t, http.StatusCreated))
	path := "/v1/admin/employees/" + employeeID + "/locations"

	if rows := data[[]officeLocation](t, sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Errorf("Pejuang baru sudah punya lokasi: %+v", rows)
	}
	// Banyak lokasi per Pejuang, duplikat digabung.
	rows := data[[]officeLocation](t, sdm.do(http.MethodPut, path, map[string]any{"location_ids": []string{a.ID, b.ID, a.ID}}).want(t, http.StatusOK))
	if len(rows) != 2 {
		t.Errorf("lokasi Pejuang = %d, ingin 2", len(rows))
	}

	// Lokasi yang masih ditugaskan tidak bisa dihapus, dan penugasan tidak berubah.
	sdm.do(http.MethodDelete, "/v1/admin/office-locations/"+a.ID, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")
	listed := data[[]officeLocation](t, sdm.do(http.MethodGet, "/v1/admin/office-locations", nil).want(t, http.StatusOK))
	if len(listed) != 2 || listed[0].EmployeeCount != 1 {
		t.Errorf("daftar lokasi: %+v", listed)
	}

	// Mengganti daftar bersifat transaksional: lokasi tak dikenal membatalkan semuanya.
	const unknown = "00000000-0000-4000-8000-000000000001"
	res := sdm.do(http.MethodPut, path, map[string]any{"location_ids": []string{b.ID, unknown}}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	if res.fields()["location_ids"] == nil {
		t.Errorf("field location_ids tidak dilaporkan: %v", res.fields())
	}
	if rows := data[[]officeLocation](t, sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK)); len(rows) != 2 {
		t.Errorf("gagal simpan mengubah penugasan: %d lokasi", len(rows))
	}
	sdm.do(http.MethodPut, path, map[string]any{"location_ids": []string{"bukan-uuid"}}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPut, path, map[string]any{}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Daftar kosong melepas semua lokasi, lalu lokasi bisa dihapus.
	if rows := data[[]officeLocation](t, sdm.do(http.MethodPut, path, map[string]any{"location_ids": []string{}}).want(t, http.StatusOK)); len(rows) != 0 {
		t.Errorf("daftar kosong tidak melepas lokasi: %+v", rows)
	}
	sdm.do(http.MethodDelete, "/v1/admin/office-locations/"+a.ID, nil).want(t, http.StatusNoContent)

	const missing = "00000000-0000-4000-8000-000000000002"
	sdm.do(http.MethodGet, "/v1/admin/employees/"+missing+"/locations", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodPut, "/v1/admin/employees/"+missing+"/locations", map[string]any{"location_ids": []string{}}).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestShiftLifecycle(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)

	body := map[string]any{"name": "Malam", "start_time": "22:00", "end_time": "06:00", "late_tolerance_minutes": 15, "work_days": []int{5, 1, 1, 3}}
	created := data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", body).want(t, http.StatusCreated))
	if created.StartTime != "22:00" || created.EndTime != "06:00" || created.LateToleranceMinutes != 15 {
		t.Errorf("shift baru: %+v", created)
	}
	if len(created.WorkDays) != 3 || created.WorkDays[0] != 1 || created.WorkDays[2] != 5 {
		t.Errorf("hari kerja harus terurut tanpa duplikat: %v", created.WorkDays)
	}

	path := "/v1/admin/shifts/" + created.ID
	updated := data[shiftRow](t, sdm.do(http.MethodPatch, path, map[string]any{"late_tolerance_minutes": 0}).want(t, http.StatusOK))
	if updated.LateToleranceMinutes != 0 || updated.Name != "Malam" || updated.StartTime != "22:00" || len(updated.WorkDays) != 3 {
		t.Errorf("PATCH mengubah field lain: %+v", updated)
	}

	cases := []struct {
		field string
		value any
	}{
		{"name", " "},
		{"start_time", "25:00"},
		{"start_time", "8:00"},
		{"end_time", "bukan-jam"},
		{"end_time", "22:00"}, // sama dengan jam masuk
		{"late_tolerance_minutes", -1},
		{"late_tolerance_minutes", 241},
		{"work_days", []int{}},
		{"work_days", []int{0}},
		{"work_days", []int{8}},
	}
	for _, c := range cases {
		res := sdm.do(http.MethodPatch, path, map[string]any{c.field: c.value}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
		if res.fields()[c.field] == nil {
			t.Errorf("%s=%v: tidak dilaporkan: %v", c.field, c.value, res.fields())
		}
	}

	// Shift yang dipakai Pejuang tidak bisa dihapus.
	employeeID := employeeInSubdivision(t, "OPS-LAP")
	exec(t, `UPDATE employees SET default_shift_id = $1 WHERE id = $2`, created.ID, employeeID)
	sdm.do(http.MethodDelete, path, nil).wantError(t, http.StatusConflict, "CONFLICT_HAS_DEPENDENTS")
	exec(t, `UPDATE employees SET default_shift_id = NULL WHERE id = $1`, employeeID)
	sdm.do(http.MethodDelete, path, nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodDelete, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

func TestHolidays(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	type holiday struct {
		Date string `json:"date"`
		Name string `json:"name"`
	}

	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2026-08-17", "Hari Kemerdekaan"}).want(t, http.StatusCreated)
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2026-12-25", "Natal"}).want(t, http.StatusCreated)
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2027-01-01", "Tahun Baru"}).want(t, http.StatusCreated)
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2026-08-17", "Dobel"}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"17-08-2026", "Salah"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2026-02-30", "Tidak ada"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPost, "/v1/admin/holidays", holiday{"2026-09-01", " "}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Batas tahun: 1 Januari 2027 tidak ikut 2026.
	rows := data[[]holiday](t, sdm.do(http.MethodGet, "/v1/admin/holidays?year=2026", nil).want(t, http.StatusOK))
	if len(rows) != 2 || rows[0].Date != "2026-08-17" || rows[1].Date != "2026-12-25" {
		t.Errorf("hari libur 2026: %+v", rows)
	}
	sdm.do(http.MethodGet, "/v1/admin/holidays", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodGet, "/v1/admin/holidays?year=1999", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	sdm.do(http.MethodDelete, "/v1/admin/holidays/2026-08-17", nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodDelete, "/v1/admin/holidays/2026-08-17", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodDelete, "/v1/admin/holidays/bukan-tanggal", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

// Hanya SDM yang boleh mengatur lokasi, shift, dan hari libur (MASTER.md §3.2).
func TestLocationShiftRequirePermission(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	employeeID := employeeInSubdivision(t, "OPS-LAP")
	acc := activeAccount(t, "pejuang", "OPS-LAP")
	pejuang := login(t, srv, acc.email, seedPassword)

	for _, route := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/v1/admin/office-locations", nil},
		{http.MethodPost, "/v1/admin/office-locations", validLocation()},
		{http.MethodGet, "/v1/admin/shifts", nil},
		{http.MethodGet, "/v1/admin/holidays?year=2026", nil},
		{http.MethodGet, "/v1/admin/employees/" + employeeID + "/locations", nil},
		{http.MethodPut, "/v1/admin/employees/" + employeeID + "/locations", map[string]any{"location_ids": []string{}}},
	} {
		pejuang.do(route.method, route.path, route.body).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	}
	if rows := data[[]officeLocation](t, sdm.do(http.MethodGet, "/v1/admin/office-locations", nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Errorf("pejuang berhasil membuat lokasi: %+v", rows)
	}
}
