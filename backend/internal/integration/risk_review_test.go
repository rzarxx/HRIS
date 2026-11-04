package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type reviewRow struct {
	ID         string   `json:"id"`
	EmployeeID string   `json:"employee_id"`
	WorkMode   string   `json:"work_mode"`
	RiskScore  int      `json:"risk_score"`
	RiskFlags  []string `json:"risk_flags"`
}

type resolvedRow struct {
	Status      string  `json:"status"`
	LateMinutes int     `json:"late_minutes"`
	ReviewNote  *string `json:"review_note"`
}

// riskyBody membuat absen dengan akurasi rendah (+15) dan jam HP selisih (+20), total 35.
func (f clockFixture) riskyBody(challenge string, lat, lon float64) map[string]any {
	body := f.clockBody(challenge, lat, lon)
	body["accuracy_m"] = 80.0
	body["captured_at"] = f.clock.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	return body
}

func (f clockFixture) riskyClockIn(t *testing.T) response {
	t.Helper()
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-in", f.riskyBody(f.challenge(t, "clock_in"), officeLat, officeLon))
}

func attemptScore(t *testing.T, employeeID string) (outcome, reason string, score int) {
	t.Helper()
	err := ownerPool.QueryRow(context.Background(), `
		SELECT outcome, reason_code, risk_score FROM attendance_attempts
		WHERE employee_id = $1 ORDER BY id DESC LIMIT 1`, employeeID).Scan(&outcome, &reason, &score)
	if err != nil {
		t.Fatal(err)
	}
	return outcome, reason, score
}

func queueFor(t *testing.T, c *client, query, employeeID string) []reviewRow {
	t.Helper()
	var out []reviewRow
	for _, r := range data[[]reviewRow](t, c.do(http.MethodGet, "/v1/admin/attendance/review"+query, nil).want(t, http.StatusOK)) {
		if r.EmployeeID == employeeID {
			out = append(out, r)
		}
	}
	return out
}

// Skor rendah tetap diterima, tetapi tercatat dan tidak masuk antrian.
func TestLowRiskIsAccepted(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	body := f.clockBody(f.challenge(t, "clock_in"), officeLat, officeLon)
	body["accuracy_m"] = 80.0
	in := data[clockInResp](t, f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body).want(t, http.StatusCreated))
	if in.Status != "present" || in.RequiresReview {
		t.Errorf("absen: %+v", in)
	}
	if outcome, reason, score := attemptScore(t, f.account.employeeID); outcome != "accepted" || reason != "ok" || score != 15 {
		t.Errorf("attempt: %s %s %d", outcome, reason, score)
	}
	if rows := queueFor(t, f.sdm, "", f.account.employeeID); len(rows) != 0 {
		t.Errorf("masuk antrian: %+v", rows)
	}
}

// Skor 30 sampai 59 masuk review, dan klien tidak pernah tahu skor atau alasannya (D8).
func TestMediumRiskGoesToReview(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	res := f.riskyClockIn(t).want(t, http.StatusCreated)
	in := data[clockInResp](t, res)
	if in.Status != "review" || !in.RequiresReview {
		t.Errorf("absen: %+v", in)
	}
	for _, leaked := range []string{"risk_score", "risk_flags", "accuracy_low", "clock_skew"} {
		if strings.Contains(string(res.body), leaked) {
			t.Errorf("respons membocorkan %s: %s", leaked, res.body)
		}
	}
	if outcome, reason, score := attemptScore(t, f.account.employeeID); outcome != "review" || reason != "risk_review" || score != 35 {
		t.Errorf("attempt: %s %s %d", outcome, reason, score)
	}
	mine := f.hp.do(http.MethodGet, "/v1/attendance", nil).want(t, http.StatusOK)
	if strings.Contains(string(mine.body), "risk_") {
		t.Errorf("riwayat membocorkan risiko: %s", mine.body)
	}

	rows := queueFor(t, f.sdm, "", f.account.employeeID)
	if len(rows) != 1 || rows[0].RiskScore != 35 {
		t.Fatalf("antrian: %+v", rows)
	}
	if got := strings.Join(rows[0].RiskFlags, ","); !strings.Contains(got, "accuracy_low") || !strings.Contains(got, "clock_skew") {
		t.Errorf("penanda: %s", got)
	}
	if rows := queueFor(t, f.sdm, "?work_mode=wfa", f.account.employeeID); len(rows) != 0 {
		t.Errorf("saring mode: %+v", rows)
	}
	if rows := queueFor(t, f.sdm, "?work_mode=onsite", f.account.employeeID); len(rows) != 1 {
		t.Errorf("saring mode onsite: %+v", rows)
	}
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/review?work_mode=ngarang", nil).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.hp.do(http.MethodGet, "/v1/admin/attendance/review", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}

// Pindah lokasi terlalu cepat bertanda location_jump, termasuk saat WFA yang melewati geofence.
func TestLocationJumpAtClockOut(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bekerja dari rumah"})
	f.clockInAt(t, officeLat, officeLon).want(t, http.StatusCreated)

	f.clock.Set(wibTime(2, 9, 0)) // sekitar 430 km dalam satu jam
	out := f.clockOutAt(t, farLat, farLon).want(t, http.StatusOK)
	if !strings.Contains(string(out.body), `"requires_review":true`) || strings.Contains(string(out.body), "location_jump") {
		t.Errorf("respons pulang: %s", out.body)
	}
	rows := queueFor(t, f.sdm, "", f.account.employeeID)
	if len(rows) != 1 || rows[0].RiskScore != 40 || rows[0].WorkMode != "wfa" {
		t.Fatalf("antrian: %+v", rows)
	}
	if !strings.Contains(strings.Join(rows[0].RiskFlags, ","), "location_jump") {
		t.Errorf("penanda: %v", rows[0].RiskFlags)
	}
}

// Skor 60 ke atas ditolak dengan pesan generik, tanpa baris absen.
func TestHighRiskIsRejected(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.grant(t, map[string]any{"mode": "wfa", "starts_on": "2026-11-02", "reason": "Bekerja dari rumah"})
	f.clockInAt(t, officeLat, officeLon).want(t, http.StatusCreated)
	f.clock.Set(wibTime(2, 9, 0))

	res := f.hp.do(http.MethodPost, "/v1/attendance/clock-out", f.riskyBody(f.challenge(t, "clock_out"), farLat, farLon))
	res.wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	if strings.Contains(string(res.body), "location_jump") || strings.Contains(string(res.body), "risk") {
		t.Errorf("respons membocorkan alasan: %s", res.body)
	}
	if outcome, reason, score := attemptScore(t, f.account.employeeID); outcome != "rejected" || reason != "risk_too_high" || score != 75 {
		t.Errorf("attempt: %s %s %d", outcome, reason, score)
	}
	// Absen masuk tetap terbuka dan statusnya tidak berubah.
	var status string
	var closed bool
	err := ownerPool.QueryRow(context.Background(), `SELECT status::text, clock_out_at IS NOT NULL FROM attendances WHERE employee_id = $1 AND work_date = '2026-11-02'`, f.account.employeeID).Scan(&status, &closed)
	if err != nil {
		t.Fatal(err)
	}
	if status != "present" || closed {
		t.Errorf("baris absen: %s, tertutup %v", status, closed)
	}
}

// Perangkat yang baru disetujui menambah poin, tetapi sendirian tidak masuk review.
func TestNewDeviceSignal(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	exec(t, `UPDATE devices SET approved_at = $1 WHERE id = $2`, f.clock.Now().Add(-time.Hour), f.deviceID)
	in := data[clockInResp](t, f.clockIn(t).want(t, http.StatusCreated))
	if in.RequiresReview {
		t.Errorf("absen: %+v", in)
	}
	if _, _, score := attemptScore(t, f.account.employeeID); score != 10 {
		t.Errorf("skor: %d", score)
	}
}

func TestResolveReview(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.clock.Set(wibTime(2, 9, 0)) // telat 60 menit
	in := data[clockInResp](t, f.riskyClockIn(t).want(t, http.StatusCreated))
	path := "/v1/admin/attendance/" + in.AttendanceID + "/resolve"

	// Validasi dan hak akses.
	f.sdm.do(http.MethodPost, path, map[string]any{"decision": "review"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.sdm.do(http.MethodPost, path, map[string]any{"decision": "absent", "note": " "}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.sdm.do(http.MethodPost, path, map[string]any{"decision": "present", "note": strings.Repeat("a", 501)}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.hp.do(http.MethodPost, path, map[string]any{"decision": "present"}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	f.sdm.do(http.MethodPost, "/v1/admin/attendance/00000000-0000-0000-0000-000000000001/resolve", map[string]any{"decision": "present"}).wantError(t, http.StatusNotFound, "NOT_FOUND")

	// Telat mempertahankan menit keterlambatan, dan keputusan tidak bisa diulang.
	late := data[resolvedRow](t, f.sdm.do(http.MethodPost, path, map[string]any{"decision": "late", "note": "Sudah dicek"}).want(t, http.StatusOK))
	if late.Status != "late" || late.LateMinutes != 60 || late.ReviewNote == nil || *late.ReviewNote != "Sudah dicek" {
		t.Errorf("keputusan telat: %+v", late)
	}
	f.sdm.do(http.MethodPost, path, map[string]any{"decision": "present"}).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")
	if rows := queueFor(t, f.sdm, "", f.account.employeeID); len(rows) != 0 {
		t.Errorf("masih di antrian: %+v", rows)
	}

	var by, action string
	if err := ownerPool.QueryRow(context.Background(), `SELECT review_by::text FROM attendances WHERE id = $1`, in.AttendanceID).Scan(&by); err != nil {
		t.Fatal(err)
	}
	if by == "" {
		t.Error("review_by kosong")
	}
	var detail []byte
	err := ownerPool.QueryRow(context.Background(), `SELECT action, detail FROM audit_logs WHERE entity_id = $1 AND action = 'attendance.review_resolved'`, in.AttendanceID).Scan(&action, &detail)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(detail, &d); err != nil {
		t.Fatal(err)
	}
	if d["decision"] != "late" || d["late_minutes_before"] != float64(60) {
		t.Errorf("audit: %v", d)
	}

	// Hadir menghapus keterlambatan.
	f.clock.Set(wibTime(3, 9, 0))
	second := data[clockInResp](t, f.riskyClockIn(t).want(t, http.StatusCreated))
	present := data[resolvedRow](t, f.sdm.do(http.MethodPost, "/v1/admin/attendance/"+second.AttendanceID+"/resolve", map[string]any{"decision": "present"}).want(t, http.StatusOK))
	if present.Status != "present" || present.LateMinutes != 0 || present.ReviewNote != nil {
		t.Errorf("keputusan hadir: %+v", present)
	}

	// Telat tidak bisa dipilih untuk absen yang tepat waktu.
	f.clock.Set(wibTime(4, 8, 0))
	third := data[clockInResp](t, f.riskyClockIn(t).want(t, http.StatusCreated))
	f.sdm.do(http.MethodPost, "/v1/admin/attendance/"+third.AttendanceID+"/resolve", map[string]any{"decision": "late"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Tidak valid: absen pulang tidak bisa lagi dipasangkan.
	absent := data[resolvedRow](t, f.sdm.do(http.MethodPost, "/v1/admin/attendance/"+third.AttendanceID+"/resolve", map[string]any{"decision": "absent", "note": "Tidak hadir"}).want(t, http.StatusOK))
	if absent.Status != "absent" || absent.LateMinutes != 0 {
		t.Errorf("keputusan tidak valid: %+v", absent)
	}
	f.clock.Set(wibTime(4, 17, 0))
	f.clockOut(t).wantError(t, http.StatusConflict, "NOT_CLOCKED_IN")
}

// Keputusan memengaruhi upah, jadi peninjau tidak boleh memutuskan absennya sendiri.
func TestResolveOwnAttendanceIsRefused(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	admin, acc := superadminClient(t, f.srv, "IT-INF")
	var id string
	err := ownerPool.QueryRow(context.Background(), `
		INSERT INTO attendances (employee_id, work_date, clock_in_at, status, risk_score)
		VALUES ($1, '2030-01-07', now(), 'review', 35) RETURNING id::text`, acc.employeeID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/admin/attendance/" + id + "/resolve"
	admin.do(http.MethodPost, path, map[string]any{"decision": "present"}).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	f.sdm.do(http.MethodPost, path, map[string]any{"decision": "present"}).want(t, http.StatusOK)
}
