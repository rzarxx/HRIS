package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// Titik jauh dari kantor untuk uji presensi di luar geofence.
const (
	farLat = -7.7956
	farLon = 110.3695
)

type arrangementRow struct {
	ID            string  `json:"id"`
	Mode          string  `json:"mode"`
	FlexibleHours bool    `json:"flexible_hours"`
	EndsOn        *string `json:"ends_on"`
	Weekdays      []int16 `json:"weekdays"`
}

type placedClockIn struct {
	WorkDate       string `json:"work_date"`
	Status         string `json:"status"`
	LateMinutes    int    `json:"late_minutes"`
	WorkMode       string `json:"work_mode"`
	OfficeLocation *struct {
		Name string `json:"name"`
	} `json:"office_location"`
}

func (f clockFixture) grant(t *testing.T, body map[string]any) arrangementRow {
	t.Helper()
	path := "/v1/admin/employees/" + f.account.employeeID + "/work-arrangements"
	return data[arrangementRow](t, f.sdm.do(http.MethodPost, path, body).want(t, http.StatusCreated))
}

func (f clockFixture) clockInAt(t *testing.T, lat, lon float64) response {
	t.Helper()
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(f.challenge(t, "clock_in"), lat, lon))
}

func (f clockFixture) clockOutAt(t *testing.T, lat, lon float64) response {
	t.Helper()
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-out", f.clockBody(f.challenge(t, "clock_out"), lat, lon))
}

// attendanceRow membaca baris presensi yang tidak semuanya dikirim ke klien.
func attendanceRow(t *testing.T, employeeID, workDate string) (mode string, hasLocation, flexible, hasCoordinate bool) {
	t.Helper()
	err := ownerPool.QueryRow(context.Background(), `
		SELECT work_mode, location_id IS NOT NULL, flexible_hours, in_latitude IS NOT NULL
		FROM attendances WHERE employee_id = $1 AND work_date = $2`, employeeID, workDate,
	).Scan(&mode, &hasLocation, &flexible, &hasCoordinate)
	if err != nil {
		t.Fatal(err)
	}
	return mode, hasLocation, flexible, hasCoordinate
}

// WFA hanya melewati geofence: GPS tetap tersimpan, telat dan sinyal keras tetap berlaku (D23).
func TestWFASkipsOnlyGeofence(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.clockInAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")

	f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bekerja dari rumah"})

	// Sinyal keras tetap ditolak walau WFA.
	mock := f.clockBody(f.challenge(t, "clock_in"), farLat, farLon)
	mock["signals"] = map[string]any{"is_mock_location": true, "app_version": "1.0.0"}
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", mock).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")

	// Telat tetap dihitung, dan lokasi kosong walau berada di dalam kantor.
	f.clock.Set(wibTime(2, 9, 0))
	in := data[placedClockIn](t, f.clockInAt(t, officeLat, officeLon).want(t, http.StatusCreated))
	if in.WorkMode != "wfa" || in.OfficeLocation != nil || in.LateMinutes != 60 || in.Status != "late" {
		t.Errorf("presensi WFA: %+v", in)
	}
	mode, hasLocation, flexible, hasCoordinate := attendanceRow(t, f.account.employeeID, "2026-11-02")
	if mode != "wfa" || hasLocation || flexible || !hasCoordinate {
		t.Errorf("baris presensi: mode %s, lokasi %v, fleksibel %v, koordinat %v", mode, hasLocation, flexible, hasCoordinate)
	}

	f.clock.Set(wibTime(2, 17, 0))
	f.clockOutAt(t, farLat, farLon).want(t, http.StatusOK)
}

// Tanpa bukti integritas presensi tetap diblokir, walau WFA.
func TestWFADoesNotBypassIntegrity(t *testing.T) {
	f := newClockFixture(t, options{strictIntegrity: true}, "08:00", "17:00")
	f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bekerja dari rumah"})
	f.clockInAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
}

// Izin dibaca dari work_date, jadi klien tidak bisa memintanya.
func TestArrangementFollowsWorkDate(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	ends := "2026-11-13"
	f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-03", "ends_on": ends, "weekdays": []int{5}, "reason": "WFA tiap Jumat"})

	// Senin belum berlaku.
	f.clockInAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	// Jumat berlaku.
	f.clock.Set(wibTime(6, 8, 0))
	f.clockInAt(t, farLat, farLon).want(t, http.StatusCreated)
	// Pulang lewat tengah malam tetap memakai izin hari masuknya, walau hari Sabtu tidak berizin.
	f.clock.Set(wibTime(7, 1, 0))
	out := data[clockOutResp](t, f.clockOutAt(t, farLat, farLon).want(t, http.StatusOK))
	if out.WorkDate != "2026-11-06" {
		t.Errorf("work_date pulang: %s", out.WorkDate)
	}
	// Lewat tanggal akhir tidak berlaku lagi.
	f.clock.Set(wibTime(20, 8, 0))
	f.clockInAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
}

// Hybrid: di dalam lokasi tercatat onsite, di luar tercatat field (D26).
func TestHybridOnsiteAndField(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.grant(t, map[string]any{"mode": "hybrid", "starts_on": "2026-11-02", "reason": "Dinas luar"})

	inside := data[placedClockIn](t, f.clockInAt(t, officeLat, officeLon).want(t, http.StatusCreated))
	if inside.WorkMode != "onsite" || inside.OfficeLocation == nil || inside.OfficeLocation.Name != "Kantor Pusat" {
		t.Errorf("hybrid di kantor: %+v", inside)
	}
	// Masuk dari kantor lalu pulang dari luar menjadi field.
	f.clock.Set(wibTime(2, 17, 0))
	f.clockOutAt(t, farLat, farLon).want(t, http.StatusOK)
	if mode, hasLocation, _, _ := attendanceRow(t, f.account.employeeID, "2026-11-02"); mode != "field" || !hasLocation {
		t.Errorf("pulang dari luar: mode %s, lokasi %v", mode, hasLocation)
	}

	// Telat tetap dihitung bila jam fleksibel tidak aktif.
	f.clock.Set(wibTime(3, 9, 0))
	outside := data[placedClockIn](t, f.clockInAt(t, farLat, farLon).want(t, http.StatusCreated))
	if outside.WorkMode != "field" || outside.OfficeLocation != nil || outside.LateMinutes != 60 {
		t.Errorf("hybrid di luar: %+v", outside)
	}
	// Masuk dari luar lalu pulang di kantor tetap field.
	f.clock.Set(wibTime(3, 17, 0))
	f.clockOutAt(t, officeLat, officeLon).want(t, http.StatusOK)
	if mode, hasLocation, _, _ := attendanceRow(t, f.account.employeeID, "2026-11-03"); mode != "field" || hasLocation {
		t.Errorf("pulang di kantor: mode %s, lokasi %v", mode, hasLocation)
	}
}

// Jam fleksibel: presensi di luar jam shift sah, tidak telat, dan tidak bertanda di luar jadwal (D26).
func TestFlexibleHours(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.grant(t, map[string]any{"mode": "hybrid", "flexible_hours": true, "starts_on": "2026-11-02", "reason": "Safari"})

	f.clock.Set(wibTime(2, 14, 0))
	in := data[placedClockIn](t, f.clockInAt(t, farLat, farLon).want(t, http.StatusCreated))
	if in.LateMinutes != 0 || in.Status != "present" || in.WorkMode != "field" {
		t.Errorf("fleksibel: %+v", in)
	}
	if _, _, flexible, _ := attendanceRow(t, f.account.employeeID, "2026-11-02"); !flexible {
		t.Error("flexible_hours tidak tersimpan")
	}

	// Sabtu bukan hari kerja shift, tetapi sah tanpa penanda.
	f.clock.Set(wibTime(7, 10, 0))
	f.clockInAt(t, officeLat, officeLon).want(t, http.StatusCreated)
	if _, _, flags := lastAttempt(t, f.account.employeeID); flags != `["unverified_integrity"]` {
		t.Errorf("penanda: %s", flags)
	}

	// Shift tetap wajib, karena menentukan work_date dan batas pulang.
	f.sdm.do(http.MethodPatch, "/v1/admin/employees/"+f.account.employeeID, map[string]any{"default_shift_id": nil}).want(t, http.StatusOK)
	f.clock.Set(wibTime(9, 10, 0))
	f.clockInAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "SHIFT_NOT_ASSIGNED")
}

// Mencabut izin langsung berlaku, dan presensi yang sudah tercatat tidak berubah.
func TestRevokeArrangementTakesEffect(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	row := f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bekerja dari rumah"})
	f.clockInAt(t, farLat, farLon).want(t, http.StatusCreated)

	path := "/v1/admin/employees/" + f.account.employeeID + "/work-arrangements"
	f.sdm.do(http.MethodDelete, path+"/"+row.ID, nil).want(t, http.StatusNoContent)
	f.sdm.do(http.MethodDelete, path+"/"+row.ID, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	if rows := data[[]arrangementRow](t, f.sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Errorf("izin masih ada: %+v", rows)
	}

	f.clock.Set(wibTime(2, 17, 0))
	f.clockOutAt(t, farLat, farLon).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.clockOutAt(t, officeLat, officeLon).want(t, http.StatusOK)
	if mode, _, _, _ := attendanceRow(t, f.account.employeeID, "2026-11-02"); mode != "wfa" {
		t.Errorf("mode presensi berubah: %s", mode)
	}

	var added, removed int
	err := ownerPool.QueryRow(context.Background(), `
		SELECT count(*) FILTER (WHERE detail->>'change' = 'work_arrangement_added'),
		       count(*) FILTER (WHERE detail->>'change' = 'work_arrangement_removed')
		FROM audit_logs WHERE action = 'employee.work_arrangement_changed' AND entity_id = $1`, f.account.employeeID,
	).Scan(&added, &removed)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || removed != 1 {
		t.Errorf("audit: ditambah %d, dicabut %d", added, removed)
	}
}

func TestArrangementRules(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	path := "/v1/admin/employees/" + f.account.employeeID + "/work-arrangements"
	valid := func(extra map[string]any) map[string]any {
		body := map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Alasan uji"}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	// Validasi.
	for name, extra := range map[string]map[string]any{
		"mode":             {"mode": "remote"},
		"tanggal":          {"starts_on": "kemarin"},
		"akhir salah":      {"ends_on": "2026-11-01"},
		"hari":             {"weekdays": []int{8}},
		"alasan":           {"reason": "   "},
		"tanpa hari aktif": {"ends_on": "2026-11-02", "weekdays": []int{5}},
	} {
		t.Run(name, func(t *testing.T) {
			f.sdm.do(http.MethodPost, path, valid(extra)).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}

	// Tumpang tindih ditolak, hari yang berbeda boleh.
	friday := f.grant(t, valid(map[string]any{"weekdays": []int{5, 5}}))
	if len(friday.Weekdays) != 1 || friday.EndsOn != nil {
		t.Errorf("izin Jumat: %+v", friday)
	}
	f.sdm.do(http.MethodPost, path, valid(nil)).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	f.sdm.do(http.MethodPost, path, valid(map[string]any{"mode": "hybrid", "weekdays": []int{5}})).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")
	f.grant(t, valid(map[string]any{"mode": "hybrid", "weekdays": []int{1, 2}}))

	// Izin tidak bisa dicabut lewat Pejuang lain.
	_, other := pejuangClient(t, f.srv, "OPS-LAP")
	f.sdm.do(http.MethodDelete, "/v1/admin/employees/"+other.employeeID+"/work-arrangements/"+friday.ID, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")

	// Pejuang tidak bisa mengatur izinnya sendiri.
	f.hp.do(http.MethodPost, path, valid(map[string]any{"weekdays": []int{3}})).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	f.hp.do(http.MethodGet, path, nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	missing := "/v1/admin/employees/00000000-0000-0000-0000-000000000001/work-arrangements"
	f.sdm.do(http.MethodGet, missing, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	f.sdm.do(http.MethodPost, missing, valid(nil)).wantError(t, http.StatusNotFound, "NOT_FOUND")
}

// Izin melewati geofence, jadi peninjau tidak boleh memberikannya untuk diri sendiri, dan SDM
// tidak boleh mengatur izin super admin (D34).
func TestArrangementSelfAndProtected(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	admin, acc := superadminClient(t, f.srv, "IT-INF")
	body := map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Alasan uji"}

	var sdmEmployee string
	if err := ownerPool.QueryRow(context.Background(), `SELECT employee_id::text FROM users WHERE role = 'sdm' LIMIT 1`).Scan(&sdmEmployee); err != nil {
		t.Fatal(err)
	}
	f.sdm.do(http.MethodPost, "/v1/admin/employees/"+sdmEmployee+"/work-arrangements", body).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	path := "/v1/admin/employees/" + acc.employeeID + "/work-arrangements"
	admin.do(http.MethodPost, path, body).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")

	// SDM ditolak mengatur izin super admin, tetapi super admin lain boleh, dan SDM boleh diatur super admin.
	f.sdm.do(http.MethodPost, path, body).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	admin2, _ := superadminClient(t, f.srv, "OPS-GDG")
	row := data[arrangementRow](t, admin2.do(http.MethodPost, path, body).want(t, http.StatusCreated))
	f.sdm.do(http.MethodDelete, path+"/"+row.ID, nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	admin2.do(http.MethodDelete, path+"/"+row.ID, nil).want(t, http.StatusNoContent)
	admin.do(http.MethodPost, "/v1/admin/employees/"+sdmEmployee+"/work-arrangements", body).want(t, http.StatusCreated)
}

// Dua penetapan bersamaan tidak boleh menghasilkan izin yang tumpang tindih.
func TestConcurrentArrangements(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	path := "/v1/admin/employees/" + f.account.employeeID + "/work-arrangements"
	var wg sync.WaitGroup
	statuses := make([]int, 4)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = f.sdm.do(http.MethodPost, path, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bersamaan"}).status
		}()
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		if s == http.StatusCreated {
			created++
		} else if s != http.StatusConflict {
			t.Errorf("status: %d", s)
		}
	}
	if created != 1 {
		t.Errorf("izin dibuat: %d", created)
	}
}
