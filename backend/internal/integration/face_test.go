package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/face"
)

// fakeFaces menggantikan dlib di tes. Isi "foto" adalah tanda JPEG diikuti "orang:geser:tanda":
// orang yang sama menghasilkan vektor dasar yang sama, dan geser memindahkannya sejauh itu,
// sehingga jarak antarfoto bisa diatur tepat.
type fakeFaces struct{}

func (fakeFaces) Model() string { return face.ModelDlib }

func (fakeFaces) Embed(_ context.Context, data []byte) ([]float64, error) {
	if !face.IsJPEG(data) {
		return nil, face.ErrBadImage
	}
	parts := strings.Split(string(data[3:]), ":")
	switch parts[0] {
	case "kosong":
		return nil, face.ErrNoFace
	case "grup":
		return nil, face.ErrManyFaces
	}
	seed, err := strconv.Atoi(parts[0])
	if err != nil || len(parts) < 2 {
		return nil, face.ErrBadImage
	}
	shift, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, face.ErrBadImage
	}
	out := make([]float64, 128)
	for i := range out {
		out[i] = 0.1*math.Sin(float64((i+1)*(seed+1))*0.37) + shift/math.Sqrt(128)
	}
	return out, nil
}

var photoSeq atomic.Int64

// photo membuat foto unik untuk orang dan pergeseran tertentu.
func photo(person string, shift float64) []byte {
	return append([]byte{0xFF, 0xD8, 0xFF}, fmt.Sprintf("%s:%g:%d", person, shift, photoSeq.Add(1))...)
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type faceChallenge struct {
	ChallengeID    string `json:"challenge_id"`
	LivenessAction string `json:"liveness_action"`
}

func (f clockFixture) faceChallenge(t *testing.T, purpose string) faceChallenge {
	t.Helper()
	f.challenge(t, purpose) // memasang X-Device-Id
	c := data[faceChallenge](t, f.hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": purpose}).want(t, http.StatusCreated))
	if c.LivenessAction == "" {
		t.Fatal("challenge tanpa liveness_action")
	}
	return c
}

func enrollSamples() [][]byte {
	return [][]byte{photo("1", 0), photo("1", 0.02), photo("1", 0.04)}
}

func (f clockFixture) enroll(t *testing.T, samples [][]byte, extra map[string]any) response {
	t.Helper()
	c := f.faceChallenge(t, "face_enroll")
	hashes := make([]string, len(samples))
	for i, s := range samples {
		hashes[i] = sha(s)
	}
	body := map[string]any{
		"challenge_id": c.ChallengeID, "sample_sha256": hashes, "liveness_passed": true, "liveness_action": c.LivenessAction,
		"consent_version": "2026-11", "latitude": officeLat, "longitude": officeLon, "accuracy_m": 10.0,
		"signals": map[string]any{"app_version": "1.0.0"},
	}
	for k, v := range extra {
		body[k] = v
	}
	return f.hp.upload("/v1/face/enroll", body, "samples", samples...)
}

func (f clockFixture) faceClock(t *testing.T, purpose string, selfie []byte, extra map[string]any) response {
	t.Helper()
	c := f.faceChallenge(t, purpose)
	body := f.clockBody(c.ChallengeID, officeLat, officeLon)
	body["selfie_sha256"] = sha(selfie)
	body["liveness_passed"] = true
	body["liveness_action"] = c.LivenessAction
	for k, v := range extra {
		body[k] = v
	}
	path := "/v1/attendance/clock-in"
	if purpose == "clock_out" {
		path = "/v1/attendance/clock-out"
	}
	return f.hp.upload(path, body, "selfie", selfie)
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

	// Selfie dikirim sebelum terdaftar.
	f.faceClock(t, "clock_in", photo("1", 0.1), nil).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED")
	if outcome, reason, _ := attemptScore(t, f.account.employeeID); outcome != "rejected" || reason != "face_not_enrolled" {
		t.Errorf("upaya: %s %s", outcome, reason)
	}

	// Pendaftaran yang tidak memenuhi syarat ditolak tanpa menyimpan template.
	field := func(res response, name string) {
		t.Helper()
		if f := res.want(t, http.StatusBadRequest).fields(); f[name] == nil {
			t.Errorf("harus ada galat %s: %v", name, f)
		}
	}
	field(f.enroll(t, [][]byte{photo("1", 0), photo("1", 0.02)}, nil), "samples")
	field(f.enroll(t, enrollSamples(), map[string]any{"consent_version": ""}), "consent_version")
	field(f.enroll(t, enrollSamples(), map[string]any{"sample_sha256": []string{"a", "b", "c"}}), "sample_sha256")
	same := photo("1", 0)
	field(f.enroll(t, [][]byte{same, same, photo("1", 0.02)}, nil), "samples")
	f.enroll(t, [][]byte{[]byte("\x89PNG"), photo("1", 0), photo("1", 0.02)}, nil).wantError(t, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
	f.enroll(t, enrollSamples(), map[string]any{"latitude": officeLat + 0.05}).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.enroll(t, enrollSamples(), map[string]any{"liveness_action": "gerakan-lain"}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	f.enroll(t, enrollSamples(), map[string]any{"liveness_passed": false}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	f.enroll(t, enrollSamples(), map[string]any{"signals": map[string]any{"root_detected": true}}).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	f.enroll(t, [][]byte{photo("1", 0), photo("kosong", 0), photo("1", 0.02)}, nil).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_DETECTED")
	field(f.enroll(t, [][]byte{photo("1", 0), photo("1", 0.02), photo("2", 0)}, nil), "samples")
	clockChallenge := f.faceChallenge(t, "clock_in").ChallengeID
	f.enroll(t, enrollSamples(), map[string]any{"challenge_id": clockChallenge}).wantError(t, http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED")
	if status().Enrolled {
		t.Fatal("pendaftaran gagal tidak boleh menyimpan template")
	}

	got := data[faceStatus](t, f.enroll(t, enrollSamples(), nil).want(t, http.StatusCreated))
	if !got.Enrolled || got.ModelVersion == nil || *got.ModelVersion != face.ModelDlib {
		t.Fatalf("daftar: %+v", got)
	}
	if !data[meFace](t, f.hp.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)).FaceEnrolled {
		t.Error("me setelah daftar")
	}
	var keyID string
	var dimension, consents int
	if err := ownerPool.QueryRow(context.Background(), `
		SELECT key_id, dimension, (SELECT count(*) FROM consents WHERE employee_id = $1 AND type = 'biometric_face')
		FROM face_templates WHERE employee_id = $1 AND is_active`, f.account.employeeID).Scan(&keyID, &dimension, &consents); err != nil {
		t.Fatal(err)
	}
	if keyID == "" || dimension != 128 || consents != 1 {
		t.Errorf("template: key %q, dimensi %d, consent %d", keyID, dimension, consents)
	}
	if countAudit(t, "face.enrolled") != 1 {
		t.Error("pendaftaran harus tercatat")
	}

	// Selfie yang tidak lolos.
	other := photo("2", 0)
	f.faceClock(t, "clock_in", other, nil).wantError(t, http.StatusUnprocessableEntity, "FACE_MISMATCH")
	f.faceClock(t, "clock_in", photo("1", 0.1), map[string]any{"liveness_action": "gerakan-lain"}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	f.faceClock(t, "clock_in", photo("grup", 0), nil).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_DETECTED")
	if f := f.faceClock(t, "clock_in", photo("1", 0.1), map[string]any{"selfie_sha256": strings.Repeat("0", 64)}).want(t, http.StatusBadRequest).fields(); f["selfie_sha256"] == nil {
		t.Errorf("hash selfie: %v", f)
	}
	// Foto yang sudah pernah dikirim ditolak sebagai replay, meski sebelumnya juga ditolak.
	f.faceClock(t, "clock_in", other, nil).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	if _, reason, _ := attemptScore(t, f.account.employeeID); reason != "embedding_replay" {
		t.Errorf("alasan replay: %s", reason)
	}

	// Jarak 0.55 dari template: diterima dengan sinyal face_borderline.
	in := data[clockInResp](t, f.faceClock(t, "clock_in", photo("1", 0.57), nil).want(t, http.StatusCreated))
	var score float64
	var flags []string
	var linked int
	if err := ownerPool.QueryRow(context.Background(), `
		SELECT face_score::float8, ARRAY(SELECT jsonb_array_elements_text(risk_flags)),
		       (SELECT count(*) FROM face_captures WHERE attendance_id = a.id)
		FROM attendances a WHERE id = $1`, in.AttendanceID).Scan(&score, &flags, &linked); err != nil {
		t.Fatal(err)
	}
	borderline := false
	for _, fl := range flags {
		borderline = borderline || fl == "face_borderline"
	}
	if math.Abs(score-0.45) > 0.001 || !borderline || in.RequiresReview || linked != 1 {
		t.Errorf("skor %v, sinyal %v, review %v, selfie terkait %d", score, flags, in.RequiresReview, linked)
	}

	f.faceClock(t, "clock_out", photo("1", 0.1), nil).want(t, http.StatusOK)
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM face_captures WHERE attendance_id = $1`, in.AttendanceID).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != 2 {
		t.Errorf("selfie masuk dan pulang terkait: %d", linked)
	}

	// Daftar ulang menggantikan template lama.
	f.enroll(t, enrollSamples(), nil).want(t, http.StatusCreated)
	var active, total int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE is_active), count(*) FROM face_templates WHERE employee_id = $1`, f.account.employeeID).Scan(&active, &total); err != nil {
		t.Fatal(err)
	}
	if active != 1 || total != 2 {
		t.Errorf("aktif %d, total %d", active, total)
	}
}

// Server tanpa model wajah menolak dengan jelas, tetapi absen development tanpa selfie tetap jalan.
func TestFaceServiceUnavailable(t *testing.T) {
	f := newClockFixture(t, options{noFaceEngine: true}, "08:00", "17:00")
	f.enroll(t, enrollSamples(), nil).wantError(t, http.StatusServiceUnavailable, "FACE_SERVICE_UNAVAILABLE")
	f.faceClock(t, "clock_in", photo("1", 0), nil).wantError(t, http.StatusServiceUnavailable, "FACE_SERVICE_UNAVAILABLE")
	f.clockIn(t).want(t, http.StatusCreated)
}

func TestFaceReset(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.enroll(t, enrollSamples(), nil).want(t, http.StatusCreated)
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
		FaceEnrolled bool `json:"face_enrolled"`
	}
	f.enroll(t, enrollSamples(), nil).want(t, http.StatusCreated)
	if one := data[employeeFace](t, f.sdm.do(http.MethodGet, "/v1/admin/employees/"+f.account.employeeID, nil).want(t, http.StatusOK)); !one.FaceEnrolled {
		t.Error("detail Pejuang harus menandai wajah terdaftar")
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
