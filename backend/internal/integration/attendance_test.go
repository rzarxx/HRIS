package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/devsim"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// fakeClock membuat aturan waktu presensi bisa diuji pasti.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func wibTime(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, pgconv.WIB)
}

const (
	officeLat = -6.2001
	officeLon = 106.8166
)

type clockFixture struct {
	srv      *httptest.Server
	clock    *fakeClock
	sdm      *client
	hp       *client
	account  account
	deviceID string
	shiftID  string
	location string
}

// newClockFixture menyiapkan Pejuang berperangkat, berlokasi, dan bershift reguler.
func newClockFixture(t *testing.T, opt options, startTime, endTime string) clockFixture {
	t.Helper()
	clock := &fakeClock{now: wibTime(2, 8, 0)} // Senin
	opt.clock = clock.Now
	opt.allowUnverified = !opt.strictIntegrity
	srv := fresh(t, opt)
	sdm := loginSDM(t, srv)
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	exec(t, `DELETE FROM attendances WHERE employee_id = $1`, acc.employeeID)

	d := data[deviceRow](t, register(hp, "utama").want(t, http.StatusCreated))
	// Perangkat lama, agar sinyal new_device_recent hanya menyala di tes yang memintanya.
	exec(t, `UPDATE devices SET approved_at = $1 WHERE id = $2`, clock.Now().AddDate(0, 0, -7), d.ID)
	loc := data[officeLocation](t, sdm.do(http.MethodPost, "/v1/admin/office-locations", map[string]any{
		"name": "Kantor Pusat", "latitude": officeLat, "longitude": officeLon, "radius_m": 100,
	}).want(t, http.StatusCreated))
	sdm.do(http.MethodPut, "/v1/admin/employees/"+acc.employeeID+"/locations", map[string]any{"location_ids": []string{loc.ID}}).want(t, http.StatusOK)
	shift := data[shiftRow](t, sdm.do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Uji", "start_time": startTime, "end_time": endTime, "late_tolerance_minutes": 10, "work_days": []int{1, 2, 3, 4, 5},
	}).want(t, http.StatusCreated))
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+acc.employeeID, map[string]any{"default_shift_id": shift.ID}).want(t, http.StatusOK)
	return clockFixture{srv: srv, clock: clock, sdm: sdm, hp: hp, account: acc, deviceID: d.ID, shiftID: shift.ID, location: loc.ID}
}

type challengeResp struct {
	ChallengeID string `json:"challenge_id"`
	Nonce       string `json:"nonce"`
}

func (f clockFixture) challenge(t *testing.T, purpose string) string {
	t.Helper()
	f.hp.headers = map[string]string{"X-Device-Id": f.deviceID}
	return data[challengeResp](t, f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": purpose}).want(t, http.StatusCreated)).ChallengeID
}

func (f clockFixture) clockBody(challenge string, lat, lon float64) map[string]any {
	return map[string]any{
		"challenge_id": challenge, "latitude": lat, "longitude": lon, "accuracy_m": 12.5,
		"captured_at": f.clock.Now().UTC().Format(time.RFC3339),
		"signals":     map[string]any{"app_version": "1.0.0"},
	}
}

func (f clockFixture) clockIn(t *testing.T) response {
	t.Helper()
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(f.challenge(t, "clock_in"), officeLat, officeLon))
}

func (f clockFixture) clockOut(t *testing.T) response {
	t.Helper()
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-out", f.clockBody(f.challenge(t, "clock_out"), officeLat, officeLon))
}

type clockInResp struct {
	AttendanceID   string `json:"attendance_id"`
	WorkDate       string `json:"work_date"`
	Status         string `json:"status"`
	LateMinutes    int    `json:"late_minutes"`
	RequiresReview bool   `json:"requires_review"`
	OfficeLocation struct {
		Name string `json:"name"`
	} `json:"office_location"`
}

type clockOutResp struct {
	WorkDate      string `json:"work_date"`
	WorkedMinutes int    `json:"worked_minutes"`
}

func lastAttempt(t *testing.T, employeeID string) (outcome, reason, flags string) {
	t.Helper()
	err := ownerPool.QueryRow(context.Background(), `
		SELECT outcome, reason_code, risk_flags::text FROM attendance_attempts
		WHERE employee_id = $1 ORDER BY id DESC LIMIT 1`, employeeID).Scan(&outcome, &reason, &flags)
	if err != nil {
		t.Fatal(err)
	}
	return outcome, reason, flags
}

func TestClockInAndOut(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")

	in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated))
	if in.WorkDate != "2026-11-02" || in.Status != "present" || in.LateMinutes != 0 || in.RequiresReview || in.OfficeLocation.Name != "Kantor Pusat" {
		t.Errorf("absen masuk: %+v", in)
	}
	if outcome, reason, flags := lastAttempt(t, f.account.employeeID); outcome != "accepted" || reason != "ok" || !strings.Contains(flags, "unverified_integrity") {
		t.Errorf("upaya masuk: %s %s %s", outcome, reason, flags)
	}
	if got := countAudit(t, "attendance.clock_in"); got != 1 {
		t.Errorf("audit clock_in = %d", got)
	}

	// Dua kali masuk ditolak.
	f.clockIn(t).wantError(t, http.StatusConflict, "ALREADY_CLOCKED_IN")

	f.clock.Set(wibTime(2, 17, 5))
	out := data[clockOutResp](t, f.clockOut(t).want(t, http.StatusOK))
	if out.WorkedMinutes != 545 || out.WorkDate != "2026-11-02" {
		t.Errorf("absen pulang: %+v", out)
	}
	f.clockOut(t).wantError(t, http.StatusConflict, "ALREADY_CLOCKED_OUT")

	// Riwayat sendiri tidak memuat skor atau alasan risiko.
	res := f.hp.do(http.MethodGet, "/v1/attendance?from=2026-11-01&to=2026-11-30", nil).want(t, http.StatusOK)
	if strings.Contains(string(res.body), "risk") {
		t.Errorf("risiko bocor ke klien: %s", res.body)
	}
	var page struct {
		Data []struct {
			WorkedMinutes *int   `json:"worked_minutes"`
			LocationName  string `json:"location_name"`
			WorkMode      string `json:"work_mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.body, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].WorkedMinutes == nil || *page.Data[0].WorkedMinutes != 545 || page.Data[0].LocationName != "Kantor Pusat" || page.Data[0].WorkMode != "onsite" {
		t.Errorf("riwayat: %s", res.body)
	}
	f.hp.do(http.MethodGet, "/v1/attendance?from=2026-01-01&to=2026-11-30", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.hp.do(http.MethodGet, "/v1/attendance?from=salah", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestClockInLateAndOffSchedule(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")

	f.clock.Set(wibTime(2, 8, 10)) // batas toleransi
	if in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated)); in.Status != "present" || in.LateMinutes != 0 {
		t.Errorf("dalam toleransi: %+v", in)
	}
	f.clock.Set(wibTime(3, 8, 30)) // Selasa
	if in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated)); in.Status != "late" || in.LateMinutes != 30 {
		t.Errorf("telat: %+v", in)
	}

	// Hari libur: tidak dihitung telat dan diberi penanda.
	f.sdm.do(http.MethodPost, "/v1/admin/holidays", map[string]any{"date": "2026-11-04", "name": "Uji"}).want(t, http.StatusCreated)
	f.clock.Set(wibTime(4, 12, 0))
	if in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated)); in.Status != "present" || in.LateMinutes != 0 {
		t.Errorf("hari libur: %+v", in)
	}
	if _, _, flags := lastAttempt(t, f.account.employeeID); !strings.Contains(flags, "off_schedule") {
		t.Errorf("penanda libur: %s", flags)
	}

	// Akhir pekan juga di luar jadwal.
	f.clock.Set(wibTime(7, 12, 0)) // Sabtu
	if in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated)); in.Status != "present" {
		t.Errorf("akhir pekan: %+v", in)
	}
}

func TestOvernightShift(t *testing.T) {
	f := newClockFixture(t, options{}, "22:00", "06:00")

	// Telat lewat tengah malam tetap masuk tanggal mulai shift.
	f.clock.Set(wibTime(3, 0, 30)) // Selasa dini hari
	in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated))
	if in.WorkDate != "2026-11-02" || in.LateMinutes != 150 || in.Status != "late" {
		t.Errorf("masuk shift malam: %+v", in)
	}
	// Pulang setelah cutoff 04.00 tetap terpasang, karena shift berakhir 06.00.
	f.clock.Set(wibTime(3, 6, 10))
	out := data[clockOutResp](t, f.clockOut(t).want(t, http.StatusOK))
	if out.WorkDate != "2026-11-02" || out.WorkedMinutes != 340 {
		t.Errorf("pulang shift malam: %+v", out)
	}
}

// Lupa absen pulang tidak diberi jam kerja otomatis (D31).
func TestMissedClockOut(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.clockOut(t).wantError(t, http.StatusConflict, "NOT_CLOCKED_IN")

	f.clockIn(t).want(t, http.StatusCreated)
	f.clock.Set(wibTime(3, 4, 0)) // tepat cutoff berikutnya
	f.clockOut(t).wantError(t, http.StatusConflict, "NOT_CLOCKED_IN")

	var worked *int
	if err := ownerPool.QueryRow(context.Background(), `SELECT worked_minutes FROM attendances WHERE employee_id = $1`, f.account.employeeID).Scan(&worked); err != nil {
		t.Fatal(err)
	}
	if worked != nil {
		t.Errorf("jam kerja terisi otomatis: %d", *worked)
	}

	// Hari berikutnya tetap bisa absen masuk.
	f.clock.Set(wibTime(3, 8, 0))
	f.clockIn(t).want(t, http.StatusCreated)
}

func TestChallengeRules(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	body := func(c string) map[string]any { return f.clockBody(c, officeLat, officeLon) }

	// Sekali pakai: walau upaya pertama gagal karena lokasi, tantangan tidak bisa diulang.
	used := f.challenge(t, "clock_in")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(used, 0, 0)).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body(used)).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")

	// Kedaluwarsa.
	old := f.challenge(t, "clock_in")
	f.clock.Set(f.clock.Now().Add(90 * time.Second))
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body(old)).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")

	// Tujuan harus sama.
	wrong := f.challenge(t, "clock_out")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body(wrong)).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")

	// Tantangan orang lain tidak bisa dipakai.
	other, _ := pejuangClient(t, f.srv, "OPS-LAP")
	otherDevice := data[deviceRow](t, register(other, "lain").want(t, http.StatusCreated))
	other.headers = map[string]string{"X-Device-Id": otherDevice.ID}
	stolen := f.challenge(t, "clock_in")
	other.do(http.MethodPost, "/v1/attendance/clock-in", body(stolen)).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")

	// Tantangan bisa dipakai pemiliknya setelah semua upaya di atas.
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body(stolen)).want(t, http.StatusCreated)

	f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "ngawur"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", map[string]any{"challenge_id": "salah"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestClockInDeviceRules(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	challenge := f.challenge(t, "clock_in")
	body := f.clockBody(challenge, officeLat, officeLon)

	f.hp.headers = nil
	f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "clock_in"}).wantError(t, http.StatusForbidden, "DEVICE_NOT_REGISTERED")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).wantError(t, http.StatusForbidden, "DEVICE_NOT_REGISTERED")

	// Perangkat milik orang lain tidak dikenali.
	other, _ := pejuangClient(t, f.srv, "OPS-LAP")
	otherDevice := data[deviceRow](t, register(other, "lain").want(t, http.StatusCreated))
	f.hp.headers = map[string]string{"X-Device-Id": otherDevice.ID}
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).wantError(t, http.StatusForbidden, "DEVICE_NOT_REGISTERED")

	// Perangkat tambahan menunggu persetujuan.
	f.hp.headers = nil
	pending := data[deviceRow](t, register(f.hp, "tambahan").want(t, http.StatusCreated))
	f.hp.headers = map[string]string{"X-Device-Id": pending.ID}
	f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "clock_in"}).wantError(t, http.StatusForbidden, "DEVICE_PENDING_APPROVAL")
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).wantError(t, http.StatusForbidden, "DEVICE_PENDING_APPROVAL")

	// Perangkat yang dicabut ditolak walau sudah memegang tantangan.
	f.sdm.do(http.MethodPost, "/v1/admin/devices/"+f.deviceID+"/revoke", nil).want(t, http.StatusOK)
	f.hp.headers = map[string]string{"X-Device-Id": f.deviceID}
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).wantError(t, http.StatusForbidden, "DEVICE_REVOKED")
	if outcome, reason, _ := lastAttempt(t, f.account.employeeID); outcome != "rejected" || reason != "device_not_approved" {
		t.Errorf("upaya: %s %s", outcome, reason)
	}
}

func TestClockInLocationAndShiftRules(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")

	// Jarak 101 meter dari pusat berradius 100 meter.
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(f.challenge(t, "clock_in"), officeLat-0.00091, officeLon)).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	if outcome, reason, _ := lastAttempt(t, f.account.employeeID); outcome != "rejected" || reason != "geofence_out" {
		t.Errorf("upaya: %s %s", outcome, reason)
	}

	// Lokasi nonaktif tidak berlaku.
	f.sdm.do(http.MethodPatch, "/v1/admin/office-locations/"+f.location, map[string]any{"is_active": false}).want(t, http.StatusOK)
	f.clockIn(t).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.sdm.do(http.MethodPatch, "/v1/admin/office-locations/"+f.location, map[string]any{"is_active": true}).want(t, http.StatusOK)

	// Banyak lokasi: cukup berada di salah satunya.
	second := data[officeLocation](t, f.sdm.do(http.MethodPost, "/v1/admin/office-locations", map[string]any{
		"name": "Cabang", "latitude": -6.9, "longitude": 107.6, "radius_m": 100,
	}).want(t, http.StatusCreated))
	f.sdm.do(http.MethodPut, "/v1/admin/employees/"+f.account.employeeID+"/locations", map[string]any{"location_ids": []string{f.location, second.ID}}).want(t, http.StatusOK)

	// Tanpa shift tidak bisa absen, dan pesannya jelas.
	f.sdm.do(http.MethodPatch, "/v1/admin/employees/"+f.account.employeeID, map[string]any{"default_shift_id": nil}).want(t, http.StatusOK)
	f.clockIn(t).wantError(t, http.StatusUnprocessableEntity, "SHIFT_NOT_ASSIGNED")
	f.sdm.do(http.MethodPatch, "/v1/admin/employees/"+f.account.employeeID, map[string]any{"default_shift_id": f.shiftID}).want(t, http.StatusOK)

	in := data[clockInResp](t, f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(f.challenge(t, "clock_in"), -6.9, 107.6)).want(t, http.StatusCreated))
	if in.OfficeLocation.Name != "Cabang" {
		t.Errorf("lokasi: %+v", in)
	}

	// Koordinat rusak ditolak sebelum menyentuh tantangan.
	bad := f.clockBody(f.challenge(t, "clock_out"), 95, 0)
	f.hp.do(http.MethodPost, "/v1/attendance/clock-out", bad).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	bad["latitude"] = officeLat
	bad["accuracy_m"] = -1
	f.hp.do(http.MethodPost, "/v1/attendance/clock-out", bad).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	bad["accuracy_m"] = 10
	bad["captured_at"] = "kemarin"
	f.hp.do(http.MethodPost, "/v1/attendance/clock-out", bad).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
}

// Sinyal keamanan ditolak tanpa memberi tahu alasannya ke klien (§5.4).
func TestHardSignalsAreGeneric(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	for key, flag := range map[string]string{
		"is_mock_location": "mock_location", "fake_gps_app_detected": "fake_gps_app",
		"root_detected": "root_detected", "emulator_detected": "emulator",
	} {
		body := f.clockBody(f.challenge(t, "clock_in"), officeLat, officeLon)
		body["signals"] = map[string]any{key: true}
		res := f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
		if strings.Contains(string(res.body), flag) {
			t.Errorf("alasan bocor: %s", res.body)
		}
		if outcome, reason, flags := lastAttempt(t, f.account.employeeID); outcome != "rejected" || reason != "hard_signal" || !strings.Contains(flags, flag) {
			t.Errorf("%s: %s %s %s", key, outcome, reason, flags)
		}
	}
	if got := countAudit(t, "attendance.rejected"); got != 4 {
		t.Errorf("audit rejected = %d", got)
	}
}

// Bawaannya semua absen ditolak selama verifikasi perangkat dan wajah belum ada (D36).
func TestUnverifiedAttendanceBlockedByDefault(t *testing.T) {
	f := newClockFixture(t, options{strictIntegrity: true}, "08:00", "17:00")
	f.clockIn(t).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	if outcome, reason, _ := lastAttempt(t, f.account.employeeID); outcome != "rejected" || reason != "integrity_failed" {
		t.Errorf("upaya: %s %s", outcome, reason)
	}
	var count int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM attendances WHERE employee_id = $1`, f.account.employeeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("absen tersimpan: %d", count)
	}
}

// Absen dibatasi 10 permintaan per menit, tanpa mengganggu riwayat.
func TestClockRateLimit(t *testing.T) {
	f := newClockFixture(t, options{limiter: auth.NewMemoryLimiter()}, "08:00", "17:00")
	f.hp.headers = map[string]string{"X-Device-Id": f.deviceID}
	for i := 0; i < 10; i++ {
		f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "clock_in"}).want(t, http.StatusCreated)
	}
	f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody("salah", officeLat, officeLon)).wantError(t, http.StatusTooManyRequests, "RATE_LIMITED")
	f.hp.do(http.MethodGet, "/v1/attendance", nil).want(t, http.StatusOK)
}

// Simulasi presensi tidak boleh menimpa jatah absen Pejuang yang sudah punya perangkat.
func TestSimulationSkipsEmployeesWithDevice(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	if _, err := devsim.FillAttendance(context.Background(), store.New(ownerPool), wibTime(2, 12, 0)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM attendances WHERE employee_id = $1`, f.account.employeeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("baris simulasi: %d", count)
	}
	f.clockIn(t).want(t, http.StatusCreated)
}

// Dua permintaan bersamaan tidak boleh sama-sama tercatat.
func TestConcurrentClockIn(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	first, second := f.challenge(t, "clock_in"), f.challenge(t, "clock_in")
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, c := range []string{first, second} {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			statuses[i] = f.hp.clone().withHeaders(f.hp.headers).do(http.MethodPost, "/v1/attendance/clock-in", f.clockBody(c, officeLat, officeLon)).status
		}(i, c)
	}
	wg.Wait()
	created, conflict := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		}
	}
	if created != 1 || conflict != 1 {
		t.Errorf("hasil: %v", statuses)
	}
}
