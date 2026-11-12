package attendance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/face"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Pencocokan wajah 1:1 di server (D3, D42, MASTER.md §5.5). HP mengirim selfie; embedding dihitung
// server, dan template tidak pernah dikirim ke klien.
const (
	PurposeFaceEnroll = "face_enroll"

	ConsentBiometricFace = "biometric_face"
	ConsentLocation      = "location"

	minFaceSamples   = 3
	maxFaceSamples   = 5
	maxConsentLength = 32

	// Skor wajah adalah kemiripan cosine SFace (D43), sama dengan face.threshold_pass dan
	// face.threshold_review (§5.4). Nilainya ada di paket face agar diuji bersama modelnya.
	FaceThresholdPass   = face.MinSimilarityPass
	FaceThresholdReview = face.MinSimilarityReview

	faceKeyID = "field-v1"

	flagFaceBorderline = "face_borderline"
	flagSelfieReplay   = "embedding_replay"
)

// LivenessActions adalah gerakan yang diminta server lewat challenge, dipilih acak agar rekaman
// video lama tidak cocok dengan permintaan. Hanya toleh kepala, karena hanya gerakan itu yang bisa
// dibuktikan server dari lima titik wajah pada foto gerakan (D50); kedip dan senyum tidak.
var LivenessActions = []string{"turn_left", "turn_right"}

// provesTurn memastikan foto gerakan membuktikan kepala menoleh (D50): foto utama lurus, foto gerakan
// menoleh cukup jauh dari foto utama, dan wajah di foto gerakan adalah orang yang sama. Arah toleh
// tidak diperiksa, sama seperti aplikasi, sampai arahnya diuji di HP sungguhan.
func provesTurn(straight, gesture face.Analysis) bool {
	return math.Abs(straight.Turn) <= face.StraightTurnMax &&
		math.Abs(gesture.Turn) >= face.GestureTurnMin &&
		math.Abs(gesture.Turn-straight.Turn) >= face.GestureTurnDelta &&
		face.Cosine(gesture.Vector, straight.Vector) >= face.GestureMinSimilarity
}

func errFaceNotEnrolled() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED", "Wajah belum terdaftar.")
}

func errLiveness() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "LIVENESS_FAILED", "Verifikasi wajah hidup gagal, silakan ulangi.")
}

func errFaceNotDetected(detail string) error {
	return httpx.NewError(http.StatusUnprocessableEntity, "FACE_NOT_DETECTED", detail)
}

func errFaceUnavailable() error {
	return httpx.NewError(http.StatusServiceUnavailable, "FACE_SERVICE_UNAVAILABLE", "Verifikasi wajah sedang tidak tersedia. Coba lagi nanti.")
}

// Cipher mengenkripsi template wajah dan selfie. Dipenuhi oleh fieldcrypt.Cipher.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

func randomLivenessAction() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(LivenessActions))))
	if err != nil {
		return "", err
	}
	return LivenessActions[n.Int64()], nil
}

// scoreOf mengubah kemiripan menjadi skor 0 sampai 1 untuk attendances.face_score.
func scoreOf(similarity float64) decimal.NullDecimal {
	score := math.Max(0, math.Min(1, similarity))
	return decimal.NullDecimal{Decimal: decimal.NewFromFloat(score).Round(4), Valid: true}
}

// faceProblem menerjemahkan kegagalan mesin wajah menjadi pesan untuk Pejuang.
func faceProblem(err error) (string, bool) {
	switch {
	case errors.Is(err, face.ErrNoFace):
		return "Wajah tidak terdeteksi. Pastikan wajah terlihat jelas dan cahaya cukup.", true
	case errors.Is(err, face.ErrManyFaces):
		return "Terdeteksi lebih dari satu wajah. Pastikan hanya wajah Anda yang terlihat.", true
	case errors.Is(err, face.ErrFaceNotCentered):
		return "Wajah terlalu kecil atau tidak di tengah bingkai", true
	case errors.Is(err, face.ErrBadImage):
		return "Foto tidak bisa dibaca. Silakan ambil ulang.", true
	}
	return "", false
}

// checkAgreement memastikan semua sampel pendaftaran saling mirip, agar satu foto yang buram atau
// milik orang lain tidak ikut membentuk template.
func checkAgreement(samples [][]float64) bool {
	for i := range samples {
		for j := i + 1; j < len(samples); j++ {
			if face.Cosine(samples[i], samples[j]) < face.MinEnrollSimilarity {
				return false
			}
		}
	}
	return true
}

// encodeEmbedding menyimpan vektor sebagai float32 little-endian sebelum dienkripsi.
func encodeEmbedding(v []float64) []byte {
	out := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(float32(x)))
	}
	return out
}

func decodeEmbedding(raw []byte) ([]float64, error) {
	if len(raw) == 0 || len(raw)%4 != 0 {
		return nil, errors.New("embedding rusak")
	}
	out := make([]float64, len(raw)/4)
	for i := range out {
		out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:])))
	}
	return out, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Service) faceReady() bool {
	return s.opt.Faces != nil && s.opt.Selfies != nil && s.opt.Cipher != nil
}

var errReplay = errors.New("selfie dipakai ulang")

// capture menyimpan selfie lalu mencatatnya. Selfie yang sama dari Pejuang yang sama ditolak
// sebagai embedding_replay, termasuk bila upaya sebelumnya ditolak.
func (s *Service) capture(ctx context.Context, employeeID uuid.UUID, challenge uuid.NullUUID, purpose string, data []byte) (uuid.UUID, error) {
	key := fmt.Sprintf("%s/%s/%s.jpg.enc", purpose, s.opt.Now().UTC().Format("2006/01"), uuid.New())
	if err := s.opt.Selfies.Put(ctx, key, data); err != nil {
		return uuid.Nil, err
	}
	id, err := s.q.InsertFaceCapture(ctx, store.InsertFaceCaptureParams{
		EmployeeID: employeeID, ChallengeID: challenge, Purpose: purpose, Sha256: sha256Hex(data), StorageKey: key,
	})
	if pgconv.IsNoRows(err) {
		if err := s.opt.Selfies.Delete(ctx, key); err != nil {
			return uuid.Nil, err
		}
		return uuid.Nil, errReplay
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// faceResult adalah hasil pencocokan wajah pada presensi.
type faceResult struct {
	capture uuid.NullUUID
	score   decimal.NullDecimal
}

// checkFace mencocokkan selfie dengan template aktif (F9). Selama verifikasi perangkat belum dibuat,
// presensi development boleh tanpa selfie (D36); begitu selfie dikirim, semuanya diperiksa.
func (s *Service) checkFace(ctx context.Context, a *attempt, in ClockInput) (faceResult, error) {
	if len(in.selfie) == 0 {
		if s.opt.AllowUnverified {
			return faceResult{}, nil
		}
		return faceResult{}, s.reject(ctx, *a, "face_not_detected", errFaceNotDetected("Selfie wajib dikirim."))
	}
	if !s.faceReady() {
		return faceResult{}, errFaceUnavailable()
	}
	tpl, err := s.activeTemplate(ctx, a.p.EmployeeID)
	if errors.Is(err, errNoTemplate) || (err == nil && tpl.modelVersion != s.opt.Faces.Model()) {
		return faceResult{}, s.reject(ctx, *a, "face_not_enrolled", errFaceNotEnrolled())
	}
	if err != nil {
		return faceResult{}, err
	}
	if in.LivenessPassed == nil || !*in.LivenessPassed || in.LivenessAction == "" || in.LivenessAction != a.livenessAction || len(in.gesture) == 0 {
		return faceResult{}, s.reject(ctx, *a, "liveness_failed", errLiveness())
	}
	captureID, err := s.capture(ctx, a.p.EmployeeID, a.challenge, a.purpose, in.selfie)
	if err == nil {
		// Foto gerakan disimpan juga agar tidak bisa dipakai ulang dan bisa ditinjau bila perlu.
		_, err = s.capture(ctx, a.p.EmployeeID, a.challenge, a.purpose, in.gesture)
	}
	if errors.Is(err, errReplay) {
		a.flags = append(a.flags, flagSelfieReplay)
		return faceResult{}, s.reject(ctx, *a, "embedding_replay", errBlocked())
	}
	if err != nil {
		return faceResult{}, err
	}
	straight, err := s.opt.Faces.Analyze(ctx, in.selfie)
	if err == nil && !straight.Centered {
		err = face.ErrFaceNotCentered
	}
	if err != nil {
		if msg, ok := faceProblem(err); ok {
			return faceResult{}, s.reject(ctx, *a, "face_not_detected", errFaceNotDetected(msg))
		}
		return faceResult{}, err
	}
	gesture, err := s.opt.Faces.Analyze(ctx, in.gesture)
	if err != nil {
		if _, ok := faceProblem(err); ok {
			return faceResult{}, s.reject(ctx, *a, "liveness_failed", errLiveness())
		}
		return faceResult{}, err
	}
	if !provesTurn(straight, gesture) {
		return faceResult{}, s.reject(ctx, *a, "liveness_failed", errLiveness())
	}
	vector := straight.Vector
	similarity := face.Cosine(vector, tpl.vector)
	out := faceResult{capture: uuid.NullUUID{UUID: captureID, Valid: true}, score: scoreOf(similarity)}
	if err := s.q.SetFaceCaptureScore(ctx, store.SetFaceCaptureScoreParams{ID: captureID, FaceScore: out.score}); err != nil {
		return faceResult{}, err
	}
	switch {
	case similarity < face.MinSimilarityReview:
		return faceResult{}, s.reject(ctx, *a, "face_mismatch", httpx.NewError(http.StatusUnprocessableEntity, "FACE_MISMATCH", "Wajah tidak cocok, silakan ulangi."))
	case similarity < face.MinSimilarityPass:
		a.flags = append(a.flags, flagFaceBorderline)
	}
	return out, nil
}

var errNoTemplate = errors.New("template wajah tidak ada")

type faceTemplate struct {
	modelVersion string
	vector       []float64
}

func (s *Service) activeTemplate(ctx context.Context, employeeID uuid.UUID) (faceTemplate, error) {
	if s.opt.Cipher == nil {
		return faceTemplate{}, errNoTemplate
	}
	row, err := s.q.GetActiveFaceTemplate(ctx, employeeID)
	if pgconv.IsNoRows(err) {
		return faceTemplate{}, errNoTemplate
	}
	if err != nil {
		return faceTemplate{}, err
	}
	plain, err := s.opt.Cipher.Decrypt(row.EmbeddingEnc)
	if err != nil {
		return faceTemplate{}, err
	}
	vector, err := decodeEmbedding(plain)
	if err != nil {
		return faceTemplate{}, err
	}
	return faceTemplate{modelVersion: row.ModelVersion, vector: vector}, nil
}

// EnrollInput adalah bagian payload pendaftaran wajah (F7). Foto sampel dikirim sebagai berkas
// `samples` dan hash-nya, berurutan, di sample_sha256.
type EnrollInput struct {
	ChallengeID    string   `json:"challenge_id"`
	CapturedAt     string   `json:"captured_at"`
	IntegrityToken string   `json:"integrity_token"`
	SampleSHA256   []string `json:"sample_sha256"`
	LivenessPassed *bool    `json:"liveness_passed"`
	LivenessAction string   `json:"liveness_action"`
	ConsentVersion string   `json:"consent_version"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	AccuracyM      *float64 `json:"accuracy_m"`
	Signals        Signals  `json:"signals"`
	// Foto saat gerakan liveness (berkas multipart `gesture`, D50).
	GestureSHA256 string `json:"gesture_sha256"`

	samples [][]byte
	gesture []byte
}

// errGestureMissing dipakai bila aplikasi lama mengirim selfie tanpa foto gerakan.
const errGestureMissing = "Foto gerakan wajib dikirim. Perbarui aplikasi lalu ulangi."

// FaceStatus adalah status wajah sendiri. Template tidak pernah dikirim.
type FaceStatus struct {
	Enrolled     bool    `json:"enrolled"`
	ModelVersion *string `json:"model_version"`
	EnrolledAt   *string `json:"enrolled_at"`
}

func (s *Service) FaceStatus(ctx context.Context, p auth.Principal) (FaceStatus, error) {
	row, err := s.q.GetActiveFaceTemplate(ctx, p.EmployeeID)
	if pgconv.IsNoRows(err) {
		return FaceStatus{}, nil
	}
	if err != nil {
		return FaceStatus{}, err
	}
	return FaceStatus{Enrolled: true, ModelVersion: &row.ModelVersion, EnrolledAt: pgconv.TimeString(row.CreatedAt)}, nil
}

func (in EnrollInput) validate() (uuid.UUID, Point, error) {
	fields := map[string]string{}
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		fields["challenge_id"] = "Tantangan tidak valid."
	}
	if v := strings.TrimSpace(in.ConsentVersion); v == "" || len(v) > maxConsentLength {
		fields["consent_version"] = "Persetujuan biometrik wajib diterima."
	}
	var point Point
	if in.Latitude == nil || in.Longitude == nil {
		fields["latitude"] = "Lokasi wajib dikirim."
	} else if point = (Point{Latitude: *in.Latitude, Longitude: *in.Longitude}); !point.Valid() {
		fields["latitude"] = "Lokasi tidak valid."
	}
	if in.AccuracyM == nil || math.IsNaN(*in.AccuracyM) || *in.AccuracyM < 0 || *in.AccuracyM > maxAccuracyM {
		fields["accuracy_m"] = "Akurasi lokasi tidak valid."
	}
	if n := len(in.samples); n < minFaceSamples || n > maxFaceSamples {
		fields["samples"] = "Kirim 3 sampai 5 foto wajah."
	} else if len(in.SampleSHA256) != n {
		fields["sample_sha256"] = "Hash foto harus sama jumlahnya dengan foto."
	} else {
		seen := map[string]bool{}
		for i, data := range in.samples {
			sum := sha256Hex(data)
			if !strings.EqualFold(in.SampleSHA256[i], sum) {
				fields["sample_sha256"] = "Hash foto tidak sesuai."
				break
			}
			if seen[sum] {
				fields["samples"] = "Setiap foto harus berbeda."
				break
			}
			seen[sum] = true
		}
	}
	if len(in.gesture) == 0 {
		fields["gesture"] = errGestureMissing
	} else if !strings.EqualFold(in.GestureSHA256, sha256Hex(in.gesture)) {
		fields["gesture_sha256"] = "Hash foto gerakan tidak sesuai."
	}
	if len(fields) > 0 {
		return challengeID, point, httpx.Validation(fields)
	}
	return challengeID, point, nil
}

// Enroll mendaftarkan wajah dari perangkat approved, di lokasi kantor (face.enroll_requires_geofence),
// lalu menggantikan template lama dalam satu transaksi.
func (s *Service) Enroll(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in EnrollInput) (FaceStatus, error) {
	challengeID, point, err := in.validate()
	if err != nil {
		return FaceStatus{}, err
	}
	if !s.faceReady() {
		return FaceStatus{}, errFaceUnavailable()
	}

	device, err := s.deviceOf(ctx, p, deviceHeader)
	if err != nil {
		return FaceStatus{}, err
	}
	challenge, err := s.q.ConsumeAttendanceChallenge(ctx, store.ConsumeAttendanceChallengeParams{
		Now: pgconv.Time(s.opt.Now()), ID: challengeID, EmployeeID: p.EmployeeID, DeviceID: device.ID, Purpose: PurposeFaceEnroll,
	})
	if err != nil {
		if pgconv.IsNoRows(err) {
			return FaceStatus{}, httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED", "Waktu pendaftaran habis, silakan ulangi.")
		}
		return FaceStatus{}, err
	}
	if len(in.Signals.hardFlags()) > 0 {
		return FaceStatus{}, errBlocked()
	}
	if in.LivenessPassed == nil || !*in.LivenessPassed || in.LivenessAction == "" || in.LivenessAction != challenge.LivenessAction.String {
		return FaceStatus{}, errLiveness()
	}
	if !s.opt.AllowUnverified {
		if s.opt.PlayVerifier == nil {
			return FaceStatus{}, errBlocked()
		}
		capturedAt, err := time.Parse(time.RFC3339Nano, in.CapturedAt)
		if err != nil {
			return FaceStatus{}, errBlocked()
		}
		hash, err := integrity.EnrollmentHash(challenge.Nonce, point.Latitude, point.Longitude, capturedAt, in.ConsentVersion, in.LivenessAction, in.SampleSHA256, in.GestureSHA256)
		if err != nil {
			return FaceStatus{}, errBlocked()
		}
		if err := s.opt.PlayVerifier.Verify(ctx, in.IntegrityToken, hash); err != nil {
			return FaceStatus{}, errBlocked()
		}
	}
	var remotePermissionID uuid.UUID
	if _, ok, err := s.matchLocation(ctx, p.EmployeeID, point); err != nil {
		return FaceStatus{}, err
	} else if !ok {
		permission, err := s.q.GetRemoteFacePermission(ctx, store.GetRemoteFacePermissionParams{EmployeeID: p.EmployeeID, Now: pgconv.Time(s.opt.Now())})
		if pgconv.IsNoRows(err) {
			return FaceStatus{}, httpx.NewError(http.StatusUnprocessableEntity, "GEOFENCE_OUT", "Daftarkan wajah di lokasi kantor atau minta izin SDM.")
		}
		if err != nil {
			return FaceStatus{}, err
		}
		remotePermissionID = permission.ID
	}

	vectors := make([][]float64, 0, len(in.samples))
	captures := make([]uuid.UUID, 0, len(in.samples))
	var first face.Analysis
	for i, data := range in.samples {
		id, err := s.capture(ctx, p.EmployeeID, uuid.NullUUID{UUID: challengeID, Valid: true}, PurposeFaceEnroll, data)
		if errors.Is(err, errReplay) {
			return FaceStatus{}, errBlocked()
		}
		if err != nil {
			return FaceStatus{}, err
		}
		captures = append(captures, id)
		a, err := s.opt.Faces.Analyze(ctx, data)
		if err == nil && !a.Centered {
			err = face.ErrFaceNotCentered
		}
		if err != nil {
			if msg, ok := faceProblem(err); ok {
				return FaceStatus{}, errFaceNotDetected(fmt.Sprintf("Foto ke-%d: %s", i+1, msg))
			}
			return FaceStatus{}, err
		}
		// Template dibentuk dari wajah lurus saja agar cocok dengan selfie presensi.
		if math.Abs(a.Turn) > face.StraightTurnMax {
			return FaceStatus{}, errFaceNotDetected(fmt.Sprintf("Foto ke-%d: wajah belum menghadap lurus ke kamera.", i+1))
		}
		if i == 0 {
			first = a
		}
		vectors = append(vectors, a.Vector)
	}
	if _, err := s.capture(ctx, p.EmployeeID, uuid.NullUUID{UUID: challengeID, Valid: true}, PurposeFaceEnroll, in.gesture); err != nil {
		if errors.Is(err, errReplay) {
			return FaceStatus{}, errBlocked()
		}
		return FaceStatus{}, err
	}
	gesture, err := s.opt.Faces.Analyze(ctx, in.gesture)
	if err != nil {
		if _, ok := faceProblem(err); ok {
			return FaceStatus{}, errLiveness()
		}
		return FaceStatus{}, err
	}
	if !provesTurn(first, gesture) {
		return FaceStatus{}, errLiveness()
	}
	if !checkAgreement(vectors) {
		return FaceStatus{}, httpx.Validation(map[string]string{"samples": "Foto wajah tidak konsisten, silakan ulangi."})
	}
	template := face.Template(vectors)
	if template == nil {
		return FaceStatus{}, httpx.Validation(map[string]string{"samples": "Foto wajah tidak valid, silakan ulangi."})
	}
	enc, err := s.opt.Cipher.Encrypt(encodeEmbedding(template))
	if err != nil {
		return FaceStatus{}, err
	}
	model := s.opt.Faces.Model()
	var row store.FaceTemplate
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if remotePermissionID != uuid.Nil {
			consumed, err := q.ConsumeRemoteFacePermission(ctx, store.ConsumeRemoteFacePermissionParams{EmployeeID: p.EmployeeID, ID: remotePermissionID, Now: pgconv.Time(s.opt.Now())})
			if err != nil {
				return err
			}
			if consumed == 0 {
				return httpx.NewError(http.StatusUnprocessableEntity, "GEOFENCE_OUT", "Izin pendaftaran jarak jauh tidak berlaku.")
			}
		}
		replaced, err := q.DeactivateFaceTemplates(ctx, store.DeactivateFaceTemplatesParams{Actor: audit.Actor(p.UserID), EmployeeID: p.EmployeeID})
		if err != nil {
			return err
		}
		if err := q.InsertConsent(ctx, store.InsertConsentParams{EmployeeID: p.EmployeeID, Type: ConsentBiometricFace, Version: strings.TrimSpace(in.ConsentVersion), Ip: ip}); err != nil {
			return err
		}
		row, err = q.InsertFaceTemplate(ctx, store.InsertFaceTemplateParams{
			EmployeeID: p.EmployeeID, DeviceID: uuid.NullUUID{UUID: device.ID, Valid: true},
			EmbeddingEnc: enc, KeyID: faceKeyID, ModelVersion: model, Dimension: int32(len(template)),
		})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.FaceEnrolled, Entity: "employee", EntityID: p.EmployeeID, IP: ip,
			Detail: map[string]any{"model_version": model, "samples": len(captures), "replaced": replaced > 0, "device_id": device.ID.String(), "remote_permission_used": remotePermissionID != uuid.Nil},
		})
	})
	if err != nil {
		return FaceStatus{}, err
	}
	return FaceStatus{Enrolled: true, ModelVersion: &row.ModelVersion, EnrolledAt: pgconv.TimeString(row.CreatedAt)}, nil
}

// ResetFace menonaktifkan template wajah seorang Pejuang agar ia mendaftar ulang. Hanya SDM, bukan
// untuk dirinya sendiri, dan tidak untuk super admin (D34).
func (s *Service) ResetFace(ctx context.Context, p auth.Principal, ip *netip.Addr, employeeID uuid.UUID, reason string) error {
	if employeeID == p.EmployeeID {
		return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Wajah sendiri harus direset peninjau lain.")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 500 {
		return httpx.Validation(map[string]string{"reason": "Alasan wajib diisi, maksimal 500 karakter."})
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.LockEmployeeForDevice(ctx, employeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, employeeID); err != nil {
			return err
		}
		n, err := q.DeactivateFaceTemplates(ctx, store.DeactivateFaceTemplatesParams{Actor: audit.Actor(p.UserID), EmployeeID: employeeID})
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NewError(http.StatusNotFound, "NOT_FOUND", "Wajah Pejuang ini belum terdaftar.")
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.FaceReset, Entity: "employee", EntityID: employeeID, IP: ip,
			Detail: map[string]any{"reason": reason},
		})
	})
}

// ConsentInput mencatat persetujuan pengguna (UU PDP, §5.6).
type ConsentInput struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

func (s *Service) RecordConsent(ctx context.Context, p auth.Principal, ip *netip.Addr, in ConsentInput) error {
	fields := map[string]string{}
	if in.Type != ConsentBiometricFace && in.Type != ConsentLocation {
		fields["type"] = "Jenis persetujuan tidak valid."
	}
	if v := strings.TrimSpace(in.Version); v == "" || len(v) > maxConsentLength {
		fields["version"] = "Versi persetujuan wajib diisi."
	}
	if len(fields) > 0 {
		return httpx.Validation(fields)
	}
	return s.q.InsertConsent(ctx, store.InsertConsentParams{EmployeeID: p.EmployeeID, Type: in.Type, Version: strings.TrimSpace(in.Version), Ip: ip})
}

func (h *Handler) FaceStatus(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.FaceStatus(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Enroll(w http.ResponseWriter, r *http.Request) error {
	var in EnrollInput
	files, err := readUploadFields(r, &in, map[string]int{"samples": maxFaceSamples, "gesture": 1})
	if err != nil {
		return err
	}
	in.samples = files["samples"]
	if f := files["gesture"]; len(f) == 1 {
		in.gesture = f[0]
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Enroll(r.Context(), p, audit.ClientIP(r), r.Header.Get(headerDeviceID), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) ResetFace(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if err := h.svc.ResetFace(r.Context(), p, audit.ClientIP(r), id, in.Reason); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Consent(w http.ResponseWriter, r *http.Request) error {
	var in ConsentInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if err := h.svc.RecordConsent(r.Context(), p, audit.ClientIP(r), in); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
