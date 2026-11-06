package attendance

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net/http"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Pencocokan wajah 1:1 di server (D3, MASTER.md §5.5). HP hanya mengirim embedding; template tidak
// pernah dikirim balik ke klien.
const (
	PurposeFaceEnroll = "face_enroll"

	ConsentBiometricFace = "biometric_face"
	ConsentLocation      = "location"

	minFaceSamples   = 3
	maxFaceSamples   = 5
	minFaceDimension = 64
	maxFaceDimension = 1024
	maxModelVersion  = 64
	maxConsentLength = 32

	// FaceThresholdPass dan FaceThresholdReview mengikuti face.threshold_pass dan face.threshold_review
	// (§5.4). Keduanya nilai awal dan dikalibrasi dengan data uji di HP sungguhan.
	FaceThresholdPass   = 0.70
	FaceThresholdReview = 0.60
	// faceSampleAgreement adalah kemiripan minimal antarsampel saat pendaftaran, agar satu sampel yang
	// buram atau milik orang lain tidak ikut membentuk template.
	faceSampleAgreement = 0.80

	faceKeyID = "field-v1"

	flagFaceBorderline = "face_borderline"
)

func errFaceNotEnrolled() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED", "Wajah belum terdaftar.")
}

func errLiveness() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "LIVENESS_FAILED", "Verifikasi wajah hidup gagal, silakan ulangi.")
}

// Cipher mengenkripsi template wajah. Dipenuhi oleh fieldcrypt.Cipher.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

// normalize mengembalikan vektor satuan, atau false bila vektornya tidak bisa dipakai.
func normalize(v []float64) ([]float64, bool) {
	var sum float64
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, false
		}
		sum += x * x
	}
	norm := math.Sqrt(sum)
	if norm == 0 || math.IsInf(norm, 0) {
		return nil, false
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out, true
}

// Cosine menghitung kemiripan dua vektor. Hasilnya 0 bila panjangnya berbeda atau salah satunya nol.
func Cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	na, okA := normalize(a)
	nb, okB := normalize(b)
	if !okA || !okB {
		return 0
	}
	var dot float64
	for i := range na {
		dot += na[i] * nb[i]
	}
	return dot
}

// BuildTemplate menggabungkan sampel menjadi satu template: rata-rata vektor satuan, lalu dinormalkan.
// Sampel harus berdimensi sama dan saling mirip.
func BuildTemplate(samples [][]float64) ([]float64, error) {
	out, problem := buildTemplate(samples)
	if problem != "" {
		return nil, httpx.Validation(map[string]string{"samples": problem})
	}
	return out, nil
}

func buildTemplate(samples [][]float64) ([]float64, string) {
	if len(samples) < minFaceSamples || len(samples) > maxFaceSamples {
		return nil, "Kirim 3 sampai 5 sampel wajah."
	}
	dim := len(samples[0])
	if dim < minFaceDimension || dim > maxFaceDimension {
		return nil, "Ukuran sampel wajah tidak valid."
	}
	units := make([][]float64, 0, len(samples))
	for _, s := range samples {
		if len(s) != dim {
			return nil, "Ukuran sampel wajah tidak sama."
		}
		u, ok := normalize(s)
		if !ok {
			return nil, "Sampel wajah tidak valid."
		}
		units = append(units, u)
	}
	for i := range units {
		for j := i + 1; j < len(units); j++ {
			if Cosine(units[i], units[j]) < faceSampleAgreement {
				return nil, "Sampel wajah tidak konsisten, silakan ulangi."
			}
		}
	}
	mean := make([]float64, dim)
	for _, u := range units {
		for i, x := range u {
			mean[i] += x
		}
	}
	out, _ := normalize(mean)
	return out, ""
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

func validModelVersion(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= maxModelVersion
}

// FaceInput adalah bagian wajah pada absen masuk dan pulang.
type FaceInput struct {
	Embedding      []float64 `json:"face_embedding"`
	ModelVersion   string    `json:"face_model_version"`
	LivenessPassed *bool     `json:"liveness_passed"`
}

func (f FaceInput) present() bool {
	return len(f.Embedding) > 0 || f.ModelVersion != "" || f.LivenessPassed != nil
}

// checkFace mencocokkan wajah dengan template aktif (F9). Selama verifikasi perangkat belum dibuat,
// absen development boleh tanpa wajah (D36); begitu bagian wajah dikirim, semuanya diperiksa.
func (s *Service) checkFace(ctx context.Context, a *attempt, in FaceInput) (decimal.NullDecimal, error) {
	if !in.present() {
		if s.opt.AllowUnverified {
			return decimal.NullDecimal{}, nil
		}
		return decimal.NullDecimal{}, s.reject(ctx, *a, "face_not_enrolled", errFaceNotEnrolled())
	}
	tpl, err := s.activeTemplate(ctx, a.p.EmployeeID)
	if errors.Is(err, errNoTemplate) || (err == nil && tpl.modelVersion != strings.TrimSpace(in.ModelVersion)) {
		return decimal.NullDecimal{}, s.reject(ctx, *a, "face_not_enrolled", errFaceNotEnrolled())
	}
	if err != nil {
		return decimal.NullDecimal{}, err
	}
	if in.LivenessPassed == nil || !*in.LivenessPassed {
		return decimal.NullDecimal{}, s.reject(ctx, *a, "liveness_failed", errLiveness())
	}
	score := Cosine(in.Embedding, tpl.vector)
	if score < FaceThresholdReview {
		return decimal.NullDecimal{}, s.reject(ctx, *a, "face_mismatch", httpx.NewError(http.StatusUnprocessableEntity, "FACE_MISMATCH", "Wajah tidak cocok, silakan ulangi."))
	}
	if score < FaceThresholdPass {
		a.flags = append(a.flags, flagFaceBorderline)
	}
	return decimal.NullDecimal{Decimal: decimal.NewFromFloat(max(score, 0)).Round(4), Valid: true}, nil
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

// EnrollInput adalah isi pendaftaran wajah (F7).
type EnrollInput struct {
	ChallengeID    string      `json:"challenge_id"`
	Samples        [][]float64 `json:"samples"`
	ModelVersion   string      `json:"face_model_version"`
	LivenessPassed *bool       `json:"liveness_passed"`
	ConsentVersion string      `json:"consent_version"`
	Latitude       *float64    `json:"latitude"`
	Longitude      *float64    `json:"longitude"`
	AccuracyM      *float64    `json:"accuracy_m"`
	Signals        Signals     `json:"signals"`
}

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

// Enroll mendaftarkan wajah dari perangkat approved, di lokasi kantor (face.enroll_requires_geofence),
// lalu menggantikan template lama dalam satu transaksi.
func (s *Service) Enroll(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in EnrollInput) (FaceStatus, error) {
	fields := map[string]string{}
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		fields["challenge_id"] = "Tantangan tidak valid."
	}
	if !validModelVersion(in.ModelVersion) {
		fields["face_model_version"] = "Versi model wajah wajib diisi."
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
	template, problem := buildTemplate(in.Samples)
	if problem != "" {
		fields["samples"] = problem
	}
	if len(fields) > 0 {
		return FaceStatus{}, httpx.Validation(fields)
	}
	if s.opt.Cipher == nil {
		return FaceStatus{}, errors.New("cipher wajah belum dipasang")
	}

	device, err := s.deviceOf(ctx, p, deviceHeader)
	if err != nil {
		return FaceStatus{}, err
	}
	if _, err := s.q.ConsumeAttendanceChallenge(ctx, store.ConsumeAttendanceChallengeParams{
		Now: pgconv.Time(s.opt.Now()), ID: challengeID, EmployeeID: p.EmployeeID, DeviceID: device.ID, Purpose: PurposeFaceEnroll,
	}); err != nil {
		if pgconv.IsNoRows(err) {
			return FaceStatus{}, httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED", "Waktu pendaftaran habis, silakan ulangi.")
		}
		return FaceStatus{}, err
	}
	if !s.opt.AllowUnverified || len(in.Signals.hardFlags()) > 0 {
		return FaceStatus{}, errBlocked()
	}
	if in.LivenessPassed == nil || !*in.LivenessPassed {
		return FaceStatus{}, errLiveness()
	}
	if _, ok, err := s.matchLocation(ctx, p.EmployeeID, point); err != nil {
		return FaceStatus{}, err
	} else if !ok {
		return FaceStatus{}, httpx.NewError(http.StatusUnprocessableEntity, "GEOFENCE_OUT", "Daftarkan wajah di lokasi kantor.")
	}

	enc, err := s.opt.Cipher.Encrypt(encodeEmbedding(template))
	if err != nil {
		return FaceStatus{}, err
	}
	model := strings.TrimSpace(in.ModelVersion)
	var row store.FaceTemplate
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
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
			Detail: map[string]any{"model_version": model, "samples": len(in.Samples), "replaced": replaced > 0, "device_id": device.ID.String()},
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
	if err := httpx.Decode(r, &in); err != nil {
		return err
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
