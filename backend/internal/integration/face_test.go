package integration

import (
	"context"
	"math"
	"net/http"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
)

const faceModel = "mobilefacenet-v1"

func faceVec(base, noise float64, seed int) []float64 {
	out := make([]float64, 128)
	for i := range out {
		out[i] = base*math.Sin(float64(i+1)) + noise*math.Cos(float64((i+1)*(seed+3)))
	}
	return out
}

func faceSamples() [][]float64 {
	return [][]float64{faceVec(1, 0.05, 1), faceVec(1, 0.05, 2), faceVec(1, 0.05, 3), faceVec(1, 0.05, 4)}
}

// probeAt membuat embedding yang kemiripannya dengan template tepat sebesar c.
func probeAt(t *testing.T, c float64) []float64 {
	t.Helper()
	tpl, err := attendance.BuildTemplate(faceSamples())
	if err != nil {
		t.Fatal(err)
	}
	other := faceVec(0, 1, 9)
	var dot, norm float64
	for i := range other {
		dot += other[i] * tpl[i]
	}
	for i := range other {
		other[i] -= dot * tpl[i]
		norm += other[i] * other[i]
	}
	norm = math.Sqrt(norm)
	out := make([]float64, len(tpl))
	for i := range out {
		out[i] = c*tpl[i] + math.Sqrt(1-c*c)*other[i]/norm
	}
	return out
}

func (f clockFixture) enroll(t *testing.T, extra map[string]any) response {
	t.Helper()
	body := map[string]any{
		"challenge_id": f.challenge(t, "face_enroll"), "samples": faceSamples(), "face_model_version": faceModel,
		"liveness_passed": true, "consent_version": "2026-11", "latitude": officeLat, "longitude": officeLon, "accuracy_m": 10.0,
		"signals": map[string]any{"app_version": "1.0.0"},
	}
	for k, v := range extra {
		body[k] = v
	}
	return f.hp.do(http.MethodPost, "/v1/face/enroll", body)
}

func (f clockFixture) faceClockIn(t *testing.T, embedding []float64, extra map[string]any) response {
	t.Helper()
	body := f.clockBody(f.challenge(t, "clock_in"), officeLat, officeLon)
	body["face_embedding"] = embedding
	body["face_model_version"] = faceModel
	body["liveness_passed"] = true
	for k, v := range extra {
		body[k] = v
	}
	return f.hp.do(http.MethodPost, "/v1/attendance/clock-in", body)
}

type faceStatus struct {
	Enrolled     bool    `json:"enrolled"`
	ModelVersion *string `json:"model_version"`
}

type meFace struct {
	FaceEnrolled bool `json:"face_enrolled"`
}

func TestFaceEnrollAndMatch(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	status := func() faceStatus {
		return data[faceStatus](t, f.hp.do(http.MethodGet, "/v1/face/status", nil).want(t, http.StatusOK))
	}
	if status().Enrolled || data[meFace](t, f.hp.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)).FaceEnrolled {
		t.Fatal("belum mendaftar")
	}

	// Wajah dikirim sebelum terdaftar.
	f.faceClockIn(t, probeAt(t, 0.9), nil).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED")
	if outcome, reason, _ := attemptScore(t, f.account.employeeID); outcome != "rejected" || reason != "face_not_enrolled" {
		t.Errorf("upaya: %s %s", outcome, reason)
	}

	// Pendaftaran yang tidak memenuhi syarat ditolak tanpa menyimpan apa pun.
	inconsistent := [][]float64{faceVec(1, 0, 1), faceVec(1, 0, 2), faceVec(0, 1, 9)}
	if fields := f.enroll(t, map[string]any{"samples": inconsistent}).want(t, http.StatusBadRequest).fields(); fields["samples"] == nil {
		t.Errorf("sampel: %v", fields)
	}
	if fields := f.enroll(t, map[string]any{"consent_version": ""}).want(t, http.StatusBadRequest).fields(); fields["consent_version"] == nil {
		t.Errorf("consent: %v", fields)
	}
	f.enroll(t, map[string]any{"latitude": officeLat + 0.05}).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.enroll(t, map[string]any{"liveness_passed": false}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	f.enroll(t, map[string]any{"signals": map[string]any{"root_detected": true}}).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	f.enroll(t, map[string]any{"challenge_id": f.challenge(t, "clock_in")}).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")
	if status().Enrolled {
		t.Fatal("pendaftaran gagal tidak boleh menyimpan template")
	}

	got := data[faceStatus](t, f.enroll(t, nil).want(t, http.StatusCreated))
	if !got.Enrolled || got.ModelVersion == nil || *got.ModelVersion != faceModel {
		t.Fatalf("daftar: %+v", got)
	}
	if !status().Enrolled || !data[meFace](t, f.hp.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)).FaceEnrolled {
		t.Error("status setelah daftar")
	}
	var keyID string
	var dimension, consents int
	var plain bool
	if err := ownerPool.QueryRow(context.Background(), `
		SELECT key_id, dimension, length(embedding_enc) = dimension * 4,
		       (SELECT count(*) FROM consents WHERE employee_id = $1 AND type = 'biometric_face')
		FROM face_templates WHERE employee_id = $1 AND is_active`, f.account.employeeID).Scan(&keyID, &dimension, &plain, &consents); err != nil {
		t.Fatal(err)
	}
	if keyID == "" || dimension != 128 || plain || consents != 1 {
		t.Errorf("template: key %q, dimensi %d, tidak terenkripsi %v, consent %d", keyID, dimension, plain, consents)
	}
	if countAudit(t, "face.enrolled") != 1 {
		t.Error("pendaftaran harus tercatat")
	}

	// Wajah berbeda, liveness gagal, dan model berbeda ditolak.
	f.faceClockIn(t, probeAt(t, 0.3), nil).wantError(t, http.StatusUnprocessableEntity, "FACE_MISMATCH")
	f.faceClockIn(t, probeAt(t, 0.95), map[string]any{"liveness_passed": false}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	f.faceClockIn(t, probeAt(t, 0.95), map[string]any{"face_model_version": "model-lain"}).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED")
	if _, reason, _ := attemptScore(t, f.account.employeeID); reason != "face_not_enrolled" {
		t.Errorf("alasan terakhir: %s", reason)
	}

	// Skor di antara ambang review dan pass: diterima dengan sinyal face_borderline.
	in := data[clockInResp](t, f.faceClockIn(t, probeAt(t, 0.65), nil).want(t, http.StatusCreated))
	var score float64
	var flags []string
	if err := ownerPool.QueryRow(context.Background(), `SELECT face_score::float8, ARRAY(SELECT jsonb_array_elements_text(risk_flags)) FROM attendances WHERE id = $1`, in.AttendanceID).Scan(&score, &flags); err != nil {
		t.Fatal(err)
	}
	borderline := false
	for _, fl := range flags {
		borderline = borderline || fl == "face_borderline"
	}
	if math.Abs(score-0.65) > 0.001 || !borderline || in.RequiresReview {
		t.Errorf("skor %v, sinyal %v, review %v", score, flags, in.RequiresReview)
	}

	body := f.clockBody(f.challenge(t, "clock_out"), officeLat, officeLon)
	body["face_embedding"], body["face_model_version"], body["liveness_passed"] = probeAt(t, 0.92), faceModel, true
	f.hp.do(http.MethodPost, "/v1/attendance/clock-out", body).want(t, http.StatusOK)

	// Daftar ulang menggantikan template lama.
	f.enroll(t, nil).want(t, http.StatusCreated)
	var active, total int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE is_active), count(*) FROM face_templates WHERE employee_id = $1`, f.account.employeeID).Scan(&active, &total); err != nil {
		t.Fatal(err)
	}
	if active != 1 || total != 2 {
		t.Errorf("aktif %d, total %d", active, total)
	}
}

func TestFaceReset(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.enroll(t, nil).want(t, http.StatusCreated)
	reset := func(c *client, id, reason string) response {
		return c.do(http.MethodPost, "/v1/admin/employees/"+id+"/face/reset", map[string]any{"reason": reason})
	}

	f.hp.do(http.MethodPost, "/v1/admin/employees/"+f.account.employeeID+"/face/reset", map[string]any{"reason": "x"}).
		wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	reset(f.sdm, f.account.employeeID, " ").want(t, http.StatusBadRequest)
	reset(f.sdm, "00000000-0000-0000-0000-000000000000", "Ganti HP").wantError(t, http.StatusNotFound, "NOT_FOUND")

	var sdmEmployee string
	if err := ownerPool.QueryRow(context.Background(), `SELECT employee_id::text FROM users WHERE email = $1`, sdmEmail).Scan(&sdmEmployee); err != nil {
		t.Fatal(err)
	}
	reset(f.sdm, sdmEmployee, "Uji").wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")

	reset(f.sdm, f.account.employeeID, "Wajah berubah setelah operasi").want(t, http.StatusNoContent)
	if data[faceStatus](t, f.hp.do(http.MethodGet, "/v1/face/status", nil).want(t, http.StatusOK)).Enrolled {
		t.Error("wajah masih terdaftar setelah reset")
	}
	if countAudit(t, "face.reset") != 1 {
		t.Error("reset harus tercatat")
	}
	reset(f.sdm, f.account.employeeID, "Ulang").wantError(t, http.StatusNotFound, "NOT_FOUND")

	type employeeFace struct {
		ID           string `json:"id"`
		FaceEnrolled bool   `json:"face_enrolled"`
	}
	f.enroll(t, nil).want(t, http.StatusCreated)
	one := data[employeeFace](t, f.sdm.do(http.MethodGet, "/v1/admin/employees/"+f.account.employeeID, nil).want(t, http.StatusOK))
	if !one.FaceEnrolled {
		t.Errorf("detail Pejuang: %+v", one)
	}
}

func TestConsents(t *testing.T) {
	srv := fresh(t, options{})
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	hp.do(http.MethodPost, "/v1/consents", map[string]any{"type": "location", "version": "2026-11"}).want(t, http.StatusNoContent)
	hp.do(http.MethodPost, "/v1/consents", map[string]any{"type": "lain", "version": "2026-11"}).want(t, http.StatusBadRequest)
	hp.do(http.MethodPost, "/v1/consents", map[string]any{"type": "location", "version": ""}).want(t, http.StatusBadRequest)
	var n int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM consents WHERE employee_id = $1 AND type = 'location'`, acc.employeeID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("consent = %d", n)
	}
}
