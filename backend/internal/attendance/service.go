package attendance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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

const (
	PurposeClockIn  = "clock_in"
	PurposeClockOut = "clock_out"

	defaultChallengeTTL = 90 * time.Second
	maxAccuracyM        = 100_000
	maxRangeDays        = 92

	// Penanda catatan tanpa poin. Skor risiko dibuat di langkah berikutnya.
	flagUnverified  = "unverified_integrity"
	flagOffSchedule = "off_schedule"
)

// Options mengatur Service.
type Options struct {
	// AllowUnverified mengizinkan presensi tanpa bukti integritas perangkat dan wajah. Hanya untuk
	// development selama verifikasinya belum dibuat; bawaannya false agar semua presensi ditolak (D36).
	AllowUnverified bool
	ChallengeTTL    time.Duration
	Now             func() time.Time
	// Cipher mengenkripsi template wajah dan selfie (§5.5).
	Cipher Cipher
	// Faces menghitung embedding dari selfie (D42) dan Selfies menyimpan fotonya. Tanpa keduanya,
	// pendaftaran dan presensi yang membawa selfie dijawab FACE_SERVICE_UNAVAILABLE.
	Faces   face.Engine
	Selfies SelfieStore
	// PlayVerifier memeriksa token pada server Google; nil mempertahankan penolakan default.
	PlayVerifier interface {
		Verify(context.Context, string, string) error
	}
	// AttestationVerifier memeriksa rantai Android Key Attestation (Jalur B, §5.4.1).
	AttestationVerifier AttestationVerifier
	// RequireDeviceKey mewajibkan setiap perangkat memiliki public_key terikat saat verifikasi aktif.
	RequireDeviceKey bool
	// Konfigurasi ambang pencocokan wajah, liveness, dan skor risiko (§5.4). Nol berarti nilai bawaan.
	FaceThresholdPass        float64
	FaceThresholdReview      float64
	FaceEnrollMinSimilarity  float64
	EnrollRequiresGeofence   *bool
	FaceStraightTurnMax      float64
	FaceGestureTurnMin       float64
	FaceGestureTurnDelta     float64
	FaceGestureMinSimilarity float64
	RiskReviewMin            int
	RiskRejectMin            int
}

// Service memuat aturan presensi (MASTER.md §5.4, alur F8 sampai F10).
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	opt  Options
}

func NewService(pool *pgxpool.Pool, opt Options) *Service {
	if opt.ChallengeTTL <= 0 {
		opt.ChallengeTTL = defaultChallengeTTL
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.FaceThresholdPass <= 0 {
		opt.FaceThresholdPass = FaceThresholdPass
	}
	if opt.FaceThresholdReview <= 0 {
		opt.FaceThresholdReview = FaceThresholdReview
	}
	if opt.FaceEnrollMinSimilarity <= 0 {
		opt.FaceEnrollMinSimilarity = face.MinEnrollSimilarity
	}
	if opt.FaceStraightTurnMax <= 0 {
		opt.FaceStraightTurnMax = face.StraightTurnMax
	}
	if opt.FaceGestureTurnMin <= 0 {
		opt.FaceGestureTurnMin = face.GestureTurnMin
	}
	if opt.FaceGestureTurnDelta <= 0 {
		opt.FaceGestureTurnDelta = face.GestureTurnDelta
	}
	if opt.FaceGestureMinSimilarity <= 0 {
		opt.FaceGestureMinSimilarity = face.GestureMinSimilarity
	}
	if opt.RiskReviewMin <= 0 {
		opt.RiskReviewMin = riskReviewMin
	}
	if opt.RiskRejectMin <= 0 {
		opt.RiskRejectMin = riskRejectMin
	}
	return &Service{pool: pool, q: store.New(pool), opt: opt}
}

func (s *Service) faceThresholdPass() float64 {
	if s.opt.FaceThresholdPass > 0 {
		return s.opt.FaceThresholdPass
	}
	return FaceThresholdPass
}

func (s *Service) faceThresholdReview() float64 {
	if s.opt.FaceThresholdReview > 0 {
		return s.opt.FaceThresholdReview
	}
	return FaceThresholdReview
}

func (s *Service) faceEnrollMinSimilarity() float64 {
	if s.opt.FaceEnrollMinSimilarity > 0 {
		return s.opt.FaceEnrollMinSimilarity
	}
	return face.MinEnrollSimilarity
}

func (s *Service) enrollRequiresGeofence() bool {
	if s.opt.EnrollRequiresGeofence != nil {
		return *s.opt.EnrollRequiresGeofence
	}
	return true
}

func (s *Service) faceStraightTurnMax() float64 {
	if s.opt.FaceStraightTurnMax > 0 {
		return s.opt.FaceStraightTurnMax
	}
	return face.StraightTurnMax
}

func (s *Service) faceGestureTurnMin() float64 {
	if s.opt.FaceGestureTurnMin > 0 {
		return s.opt.FaceGestureTurnMin
	}
	return face.GestureTurnMin
}

func (s *Service) faceGestureTurnDelta() float64 {
	if s.opt.FaceGestureTurnDelta > 0 {
		return s.opt.FaceGestureTurnDelta
	}
	return face.GestureTurnDelta
}

func (s *Service) faceGestureMinSimilarity() float64 {
	if s.opt.FaceGestureMinSimilarity > 0 {
		return s.opt.FaceGestureMinSimilarity
	}
	return face.GestureMinSimilarity
}

func (s *Service) riskReviewMin() int {
	if s.opt.RiskReviewMin > 0 {
		return s.opt.RiskReviewMin
	}
	return riskReviewMin
}

func (s *Service) riskRejectMin() int {
	if s.opt.RiskRejectMin > 0 {
		return s.opt.RiskRejectMin
	}
	return riskRejectMin
}

// Signals adalah laporan dari HP. Sifatnya saran, tetapi laporan buruk tetap ditolak (§5.4).
type Signals struct {
	IsMockLocation     bool   `json:"is_mock_location"`
	FakeGPSAppDetected bool   `json:"fake_gps_app_detected"`
	RootDetected       bool   `json:"root_detected"`
	EmulatorDetected   bool   `json:"emulator_detected"`
	AppVersion         string `json:"app_version"`
}

func (s Signals) hardFlags() []string {
	var flags []string
	if s.IsMockLocation {
		flags = append(flags, "mock_location")
	}
	if s.FakeGPSAppDetected {
		flags = append(flags, "fake_gps_app")
	}
	if s.RootDetected {
		flags = append(flags, "root_detected")
	}
	if s.EmulatorDetected {
		flags = append(flags, "emulator")
	}
	return flags
}

// ClockInput adalah isi presensi masuk dan pulang.
type ClockInput struct {
	ChallengeID      string   `json:"challenge_id"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	AccuracyM        *float64 `json:"accuracy_m"`
	CapturedAt       string   `json:"captured_at"`
	IntegrityToken   string   `json:"integrity_token"`
	DeviceProof      string   `json:"device_proof"`
	AttestationChain []string `json:"attestation_chain"`
	Signals          Signals  `json:"signals"`
	// Bagian wajah (D42). Selfie dikirim sebagai berkas multipart `selfie`; hash-nya di payload
	// mengikat foto ke challenge.
	SelfieSHA256   string `json:"selfie_sha256"`
	LivenessPassed *bool  `json:"liveness_passed"`
	LivenessAction string `json:"liveness_action"`
	// Foto saat gerakan liveness (berkas multipart `gesture`), dipakai server untuk membuktikan
	// kepala benar-benar menoleh (D50).
	GestureSHA256 string `json:"gesture_sha256"`

	selfie  []byte
	gesture []byte
}

type parsedClock struct {
	challengeID uuid.UUID
	point       Point
	accuracyM   float64
	capturedAt  time.Time
}

func (in ClockInput) parse() (parsedClock, error) {
	fields := map[string]string{}
	var p parsedClock
	id, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		fields["challenge_id"] = "Tantangan tidak valid."
	}
	p.challengeID = id
	if in.Latitude == nil || in.Longitude == nil {
		fields["latitude"] = "Lokasi wajib dikirim."
	} else {
		p.point = Point{Latitude: *in.Latitude, Longitude: *in.Longitude}
		if !p.point.Valid() {
			fields["latitude"] = "Lokasi tidak valid."
		}
	}
	if in.AccuracyM == nil || math.IsNaN(*in.AccuracyM) || *in.AccuracyM < 0 || *in.AccuracyM > maxAccuracyM {
		fields["accuracy_m"] = "Akurasi lokasi tidak valid."
	} else {
		p.accuracyM = *in.AccuracyM
	}
	captured, err := time.Parse(time.RFC3339, in.CapturedAt)
	if err != nil {
		fields["captured_at"] = "Waktu pengambilan tidak valid."
	}
	p.capturedAt = captured
	if len(in.selfie) > 0 && !strings.EqualFold(in.SelfieSHA256, sha256Hex(in.selfie)) {
		fields["selfie_sha256"] = "Hash selfie tidak sesuai."
	}
	if len(in.selfie) > 0 && len(in.gesture) == 0 {
		fields["gesture"] = errGestureMissing
	} else if len(in.gesture) > 0 && !strings.EqualFold(in.GestureSHA256, sha256Hex(in.gesture)) {
		fields["gesture_sha256"] = "Hash foto gerakan tidak sesuai."
	}
	if len(fields) > 0 {
		return p, httpx.Validation(fields)
	}
	return p, nil
}

func errBlocked() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED", "Presensi ditolak. Hubungi SDM.")
}

func coordinate(v float64) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: decimal.NewFromFloat(v).Round(6), Valid: true}
}

func scheduleOf(s store.Shift) Schedule {
	return Schedule{
		Start:            time.Duration(s.StartTime.Microseconds) * time.Microsecond,
		End:              time.Duration(s.EndTime.Microseconds) * time.Microsecond,
		ToleranceMinutes: int(s.LateToleranceMinutes),
		WorkDays:         s.WorkDays,
	}
}

// deviceOf memastikan X-Device-Id adalah perangkat approved milik pengguna ini.
func (s *Service) deviceOf(ctx context.Context, p auth.Principal, header string) (store.Device, error) {
	id, err := uuid.Parse(header)
	if err != nil {
		return store.Device{}, httpx.NewError(http.StatusForbidden, "DEVICE_NOT_REGISTERED", "Perangkat belum terdaftar.")
	}
	// Presensi hanya sah dari sesi aplikasi yang terikat perangkat. Sesi web tidak punya ikatan itu,
	// sehingga X-Device-Id bisa diisi siapa saja yang tahu kata sandinya (D39).
	if p.DeviceID == uuid.Nil && !s.opt.AllowUnverified {
		return store.Device{}, httpx.NewError(http.StatusForbidden, "DEVICE_NOT_REGISTERED", "Perangkat belum terdaftar.")
	}
	if p.DeviceID != uuid.Nil && p.DeviceID != id {
		return store.Device{}, httpx.NewError(http.StatusForbidden, "DEVICE_NOT_REGISTERED", "Perangkat belum terdaftar.")
	}
	device, err := s.q.GetOwnDevice(ctx, store.GetOwnDeviceParams{ID: id, EmployeeID: p.EmployeeID})
	if pgconv.IsNoRows(err) {
		return device, httpx.NewError(http.StatusForbidden, "DEVICE_NOT_REGISTERED", "Perangkat belum terdaftar.")
	}
	if err != nil {
		return device, err
	}
	switch device.Status {
	case store.DeviceStatusApproved:
		return device, nil
	case store.DeviceStatusPending:
		return device, httpx.NewError(http.StatusForbidden, "DEVICE_PENDING_APPROVAL", "Perangkat menunggu persetujuan.")
	default:
		return device, httpx.NewError(http.StatusForbidden, "DEVICE_REVOKED", "Perangkat ini sudah dicabut. Hubungi SDM.")
	}
}

// Challenge adalah jawaban permintaan tantangan.
type Challenge struct {
	ChallengeID string `json:"challenge_id"`
	Nonce       string `json:"nonce"`
	ExpiresAt   string `json:"expires_at"`
	// LivenessAction adalah gerakan yang harus dilakukan Pejuang saat liveness (D42).
	LivenessAction string `json:"liveness_action"`
}

// NewChallenge membuat tantangan sekali pakai yang terikat ke pengguna dan perangkat.
func (s *Service) NewChallenge(ctx context.Context, p auth.Principal, deviceHeader, purpose string) (Challenge, error) {
	if purpose != PurposeClockIn && purpose != PurposeClockOut && purpose != PurposeFaceEnroll {
		return Challenge{}, httpx.Validation(map[string]string{"purpose": "Tujuan tidak valid."})
	}
	device, err := s.deviceOf(ctx, p, deviceHeader)
	if err != nil {
		return Challenge{}, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Challenge{}, err
	}
	action, err := randomLivenessAction()
	if err != nil {
		return Challenge{}, err
	}
	row, err := s.q.CreateAttendanceChallenge(ctx, store.CreateAttendanceChallengeParams{
		EmployeeID: p.EmployeeID, DeviceID: device.ID, Nonce: hex.EncodeToString(raw), Purpose: purpose,
		ExpiresAt: pgconv.Time(s.opt.Now().Add(s.opt.ChallengeTTL)), LivenessAction: pgtype.Text{String: action, Valid: true},
	})
	if err != nil {
		return Challenge{}, err
	}
	return Challenge{
		ChallengeID: row.ID.String(), Nonce: row.Nonce, ExpiresAt: row.ExpiresAt.Time.UTC().Format(time.RFC3339),
		LivenessAction: row.LivenessAction.String,
	}, nil
}

// attempt mengumpulkan data satu upaya presensi untuk attendance_attempts.
type attempt struct {
	p         auth.Principal
	ip        *netip.Addr
	purpose   string
	device    uuid.NullUUID
	challenge uuid.NullUUID
	point     Point
	flags     []string
	score     int
	// approvedAt adalah waktu perangkat disetujui, untuk sinyal new_device_recent.
	approvedAt pgtype.Timestamptz
	// livenessAction adalah gerakan yang diminta challenge ini.
	livenessAction string
}

func (a attempt) params(outcome, reason string) (store.InsertAttendanceAttemptParams, error) {
	flags := a.flags
	if flags == nil {
		flags = []string{}
	}
	raw, err := json.Marshal(flags)
	if err != nil {
		return store.InsertAttendanceAttemptParams{}, err
	}
	return store.InsertAttendanceAttemptParams{
		EmployeeID: a.p.EmployeeID, DeviceID: a.device, ChallengeID: a.challenge, Purpose: a.purpose,
		Outcome: outcome, ReasonCode: reason, RiskScore: int32(a.score), RiskFlags: raw,
		Latitude: coordinate(a.point.Latitude), Longitude: coordinate(a.point.Longitude), Ip: a.ip,
	}, nil
}

// reject mencatat upaya yang ditolak lalu mengembalikan error untuk klien. Pencatatan berada di
// luar transaksi presensi agar tetap tersimpan, dan alasan rinci hanya terlihat oleh SDM.
func (s *Service) reject(ctx context.Context, a attempt, reason string, out error) error {
	params, err := a.params("rejected", reason)
	if err != nil {
		return err
	}
	if err := s.q.InsertAttendanceAttempt(ctx, params); err != nil {
		return err
	}
	if err := audit.Write(ctx, s.q, audit.Entry{
		UserID: audit.Actor(a.p.UserID), Action: audit.AttendanceRejected, Entity: "employee", EntityID: a.p.EmployeeID,
		Detail: map[string]any{"purpose": a.purpose, "reason": reason, "flags": a.flags}, IP: a.ip,
	}); err != nil {
		return err
	}
	return out
}

// verify menjalankan pemeriksaan yang sama untuk presensi masuk dan pulang (F9). Geofence diperiksa
// sesudahnya oleh place, karena izin kerja bergantung pada work_date.
func (s *Service) verify(ctx context.Context, a *attempt, deviceHeader string, in ClockInput, parsed parsedClock) error {
	device, err := s.deviceOf(ctx, a.p, deviceHeader)
	if err != nil {
		if device.ID != uuid.Nil {
			a.device = uuid.NullUUID{UUID: device.ID, Valid: true}
		}
		return s.reject(ctx, *a, "device_not_approved", err)
	}
	a.device = uuid.NullUUID{UUID: device.ID, Valid: true}
	a.approvedAt = device.ApprovedAt

	challenge, err := s.q.ConsumeAttendanceChallenge(ctx, store.ConsumeAttendanceChallengeParams{
		Now: pgconv.Time(s.opt.Now()), ID: parsed.challengeID, EmployeeID: a.p.EmployeeID, DeviceID: device.ID, Purpose: a.purpose,
	})
	if err != nil {
		if pgconv.IsNoRows(err) {
			return s.reject(ctx, *a, "challenge_expired", httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED", "Waktu presensi habis, silakan ulangi."))
		}
		return err
	}
	a.challenge = uuid.NullUUID{UUID: parsed.challengeID, Valid: true}
	a.livenessAction = challenge.LivenessAction.String

	if hard := in.Signals.hardFlags(); len(hard) > 0 {
		a.flags = append(a.flags, hard...)
		return s.reject(ctx, *a, "hard_signal", errBlocked())
	}
	if s.opt.AllowUnverified {
		a.flags = append(a.flags, flagUnverified)
		return nil
	}
	if len(in.selfie) == 0 {
		a.flags = append(a.flags, flagIntegrityFailed)
		return s.reject(ctx, *a, flagIntegrityFailed, errBlocked())
	}
	hash, err := integrity.AttendanceHash(challenge.Nonce, parsed.point.Latitude, parsed.point.Longitude, parsed.capturedAt, in.SelfieSHA256, in.GestureSHA256)
	if err != nil {
		a.flags = append(a.flags, flagIntegrityFailed)
		return s.reject(ctx, *a, flagIntegrityFailed, errBlocked())
	}
	softFlags, reasonCode, ok := s.verifyDeviceIntegrity(ctx, device, hash, in.IntegrityToken, in.DeviceProof, in.AttestationChain, true)
	if !ok {
		a.flags = append(a.flags, reasonCode)
		return s.reject(ctx, *a, reasonCode, errBlocked())
	}
	a.flags = append(a.flags, softFlags...)
	return nil
}

// assess menilai sinyal lunak (§5.4). Skor tinggi ditolak dengan pesan generik, karena klien tidak
// boleh tahu alasannya (D8). Skor menengah tetap tercatat dan masuk antrian Review Presensi.
func (s *Service) assess(ctx context.Context, a *attempt, parsed parsedClock, now time.Time) error {
	input := RiskInput{Now: now, Point: parsed.point, AccuracyM: parsed.accuracyM, CapturedAt: parsed.capturedAt}
	if a.approvedAt.Valid {
		input.DeviceApprovedAt = &a.approvedAt.Time
	}
	row, err := s.q.GetLastAttendancePoint(ctx, a.p.EmployeeID)
	switch {
	case err == nil:
		input.Previous = previousPoint(row)
	case !pgconv.IsNoRows(err):
		return err
	}
	a.flags = append(a.flags, SoftFlags(input)...)
	// liveness_retry dihitung server dari upaya yang ditolak, bukan dari laporan aplikasi.
	failures, err := s.q.CountRecentFaceFailures(ctx, store.CountRecentFaceFailuresParams{
		EmployeeID: a.p.EmployeeID, Purpose: a.purpose, Since: pgconv.Time(now.Add(-livenessRetryWindow)),
	})
	if err != nil {
		return err
	}
	if LivenessRetried(failures) {
		a.flags = append(a.flags, flagLivenessRetry)
	}
	a.score = RiskScore(a.flags)
	if a.score >= s.riskRejectMin() {
		return s.reject(ctx, *a, "risk_too_high", errBlocked())
	}
	return nil
}

func previousPoint(r store.GetLastAttendancePointRow) *PointAt {
	if r.ClockOutAt.Valid && r.OutLatitude.Valid && r.OutLongitude.Valid {
		return &PointAt{Point: Point{Latitude: r.OutLatitude.Decimal.InexactFloat64(), Longitude: r.OutLongitude.Decimal.InexactFloat64()}, At: r.ClockOutAt.Time}
	}
	if r.ClockInAt.Valid && r.InLatitude.Valid && r.InLongitude.Valid {
		return &PointAt{Point: Point{Latitude: r.InLatitude.Decimal.InexactFloat64(), Longitude: r.InLongitude.Decimal.InexactFloat64()}, At: r.ClockInAt.Time}
	}
	return nil
}

// recorded mengembalikan baris attempt untuk upaya yang tercatat, sesuai skornya.
func (a attempt) recorded(reviewMin int) (store.InsertAttendanceAttemptParams, error) {
	if reviewMin <= 0 {
		reviewMin = riskReviewMin
	}
	if a.score >= reviewMin {
		return a.params("review", "risk_review")
	}
	return a.params("accepted", "ok")
}

func (s *Service) matchLocation(ctx context.Context, employeeID uuid.UUID, p Point) (store.ListEmployeeLocationsRow, bool, error) {
	rows, err := s.q.ListEmployeeLocations(ctx, employeeID)
	if err != nil {
		return store.ListEmployeeLocationsRow{}, false, err
	}
	for _, r := range rows {
		if !r.IsActive {
			continue
		}
		area := Area{
			Center:  Point{Latitude: r.Latitude.InexactFloat64(), Longitude: r.Longitude.InexactFloat64()},
			RadiusM: int(r.RadiusM),
		}
		if len(r.Polygon) > 0 {
			if err := json.Unmarshal(r.Polygon, &area.Polygon); err != nil {
				return r, false, err
			}
		}
		if area.Contains(p) {
			return r, true, nil
		}
	}
	return store.ListEmployeeLocationsRow{}, false, nil
}

// LocationRef adalah ringkasan lokasi di respons.
type LocationRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClockInResult adalah jawaban presensi masuk. Skor dan alasan risiko tidak pernah dikirim ke klien.
type ClockInResult struct {
	AttendanceID   string       `json:"attendance_id"`
	WorkDate       string       `json:"work_date"`
	Status         string       `json:"status"`
	ClockInAt      string       `json:"clock_in_at"`
	LateMinutes    int          `json:"late_minutes"`
	WorkMode       string       `json:"work_mode"`
	OfficeLocation *LocationRef `json:"office_location"` // kosong saat WFA dan field
	RequiresReview bool         `json:"requires_review"`
}

// ClockIn mencatat presensi masuk.
func (s *Service) ClockIn(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in ClockInput) (ClockInResult, error) {
	parsed, err := in.parse()
	if err != nil {
		return ClockInResult{}, err
	}
	a := attempt{p: p, ip: ip, purpose: PurposeClockIn, point: parsed.point}
	if err := s.verify(ctx, &a, deviceHeader, in, parsed); err != nil {
		return ClockInResult{}, err
	}

	shift, err := s.q.GetEmployeeDefaultShift(ctx, p.EmployeeID)
	if pgconv.IsNoRows(err) {
		return ClockInResult{}, s.reject(ctx, a, "shift_not_assigned", httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_ASSIGNED", "Shift Anda belum ditentukan. Hubungi SDM."))
	}
	if err != nil {
		return ClockInResult{}, err
	}

	now := s.opt.Now()
	schedule := scheduleOf(shift)
	workDate := WorkDate(now, schedule)
	// Cuti/izin yang disetujui sudah mencatat hari ini (F13); beri alasan yang jelas, bukan
	// "sudah presensi masuk".
	if day, err := s.q.GetAttendanceStatusOn(ctx, store.GetAttendanceStatusOnParams{EmployeeID: p.EmployeeID, WorkDate: pgconv.Date(workDate)}); err == nil &&
		day.Status == store.AttStatusLeave && !day.ClockInAt.Valid {
		return ClockInResult{}, s.reject(ctx, a, "on_leave", httpx.NewError(http.StatusConflict, "LEAVE_ACTIVE", "Hari ini Anda tercatat cuti atau izin, jadi tidak perlu presensi."))
	} else if err != nil && !pgconv.IsNoRows(err) {
		return ClockInResult{}, err
	}
	where, err := s.place(ctx, a, workDate, parsed.point)
	if err != nil {
		return ClockInResult{}, err
	}
	faces, err := s.checkFace(ctx, &a, in)
	if err != nil {
		return ClockInResult{}, err
	}
	if err := s.assess(ctx, &a, parsed, now); err != nil {
		return ClockInResult{}, err
	}
	holiday, err := s.q.IsHoliday(ctx, pgconv.Date(workDate))
	if err != nil {
		return ClockInResult{}, err
	}
	status := store.AttStatusPresent
	late := 0
	switch {
	case where.flexible:
		// Jam fleksibel: presensi di luar jam shift sah dan tidak dihitung telat (D26).
	case holiday || !IsWorkDay(workDate, schedule):
		a.flags = append(a.flags, flagOffSchedule)
	default:
		if late = LateMinutes(now, workDate, schedule); late > 0 {
			status = store.AttStatusLate
		}
	}
	if a.score >= s.riskReviewMin() {
		status = store.AttStatusReview
	}

	accepted, err := a.recorded(s.riskReviewMin())
	if err != nil {
		return ClockInResult{}, err
	}
	var row store.Attendance
	conflict := false
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		row, err = q.ClockIn(ctx, store.ClockInParams{
			EmployeeID: p.EmployeeID, WorkDate: pgconv.Date(workDate), ShiftID: uuid.NullUUID{UUID: shift.ID, Valid: true},
			ClockInAt:   pgconv.Time(now),
			InAccuracyM: decimal.NullDecimal{Decimal: decimal.NewFromFloat(parsed.accuracyM).Round(2), Valid: true},
			RiskScore:   accepted.RiskScore, RiskFlags: accepted.RiskFlags, Status: status, LateMinutes: int32(late),
			WorkMode: where.workMode, FlexibleHours: where.flexible, LocationID: where.locationID(),
			InLatitude: coordinate(parsed.point.Latitude), InLongitude: coordinate(parsed.point.Longitude),
			FaceScore: faces.score,
		})
		if pgconv.IsNoRows(err) {
			conflict = true
			return nil
		}
		if err != nil {
			return err
		}
		if err := linkCapture(ctx, q, faces, row.ID); err != nil {
			return err
		}
		if err := q.InsertAttendanceAttempt(ctx, accepted); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AttendanceClockIn, Entity: "attendance", EntityID: row.ID, IP: ip,
		})
	})
	if err != nil {
		return ClockInResult{}, err
	}
	if conflict {
		return ClockInResult{}, s.reject(ctx, a, "already_clocked_in", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_IN", "Anda sudah presensi masuk."))
	}
	return ClockInResult{
		AttendanceID: row.ID.String(), WorkDate: workDate.Format("2006-01-02"), Status: string(row.Status),
		ClockInAt: row.ClockInAt.Time.UTC().Format(time.RFC3339), LateMinutes: late, WorkMode: row.WorkMode,
		OfficeLocation: where.locationRef(), RequiresReview: row.Status == store.AttStatusReview,
	}, nil
}

// ClockOutResult adalah jawaban presensi pulang.
type ClockOutResult struct {
	AttendanceID   string `json:"attendance_id"`
	WorkDate       string `json:"work_date"`
	ClockOutAt     string `json:"clock_out_at"`
	WorkedMinutes  int    `json:"worked_minutes"`
	RequiresReview bool   `json:"requires_review"`
}

func (s *Service) deadlineOf(ctx context.Context, row store.Attendance) (time.Time, error) {
	if !row.ShiftID.Valid {
		return CloseDeadline(row.WorkDate.Time, nil), nil
	}
	shift, err := s.q.GetShift(ctx, row.ShiftID.UUID)
	if pgconv.IsNoRows(err) {
		return CloseDeadline(row.WorkDate.Time, nil), nil
	}
	if err != nil {
		return time.Time{}, err
	}
	schedule := scheduleOf(shift)
	return CloseDeadline(row.WorkDate.Time, &schedule), nil
}

// ClockOut mencatat presensi pulang pada presensi masuk yang masih terbuka (F10).
func (s *Service) ClockOut(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in ClockInput) (ClockOutResult, error) {
	parsed, err := in.parse()
	if err != nil {
		return ClockOutResult{}, err
	}
	a := attempt{p: p, ip: ip, purpose: PurposeClockOut, point: parsed.point}
	if err := s.verify(ctx, &a, deviceHeader, in, parsed); err != nil {
		return ClockOutResult{}, err
	}

	now := s.opt.Now()
	notClockedIn := func() error {
		last, err := s.q.GetLastClosedAttendance(ctx, p.EmployeeID)
		if err == nil {
			deadline, err := s.deadlineOf(ctx, last)
			if err != nil {
				return err
			}
			if now.Before(deadline) {
				return s.reject(ctx, a, "already_clocked_out", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_OUT", "Anda sudah presensi pulang."))
			}
		} else if !pgconv.IsNoRows(err) {
			return err
		}
		return s.reject(ctx, a, "not_clocked_in", httpx.NewError(http.StatusConflict, "NOT_CLOCKED_IN", "Anda belum presensi masuk."))
	}

	open, err := s.q.GetOpenAttendance(ctx, p.EmployeeID)
	if pgconv.IsNoRows(err) {
		return ClockOutResult{}, notClockedIn()
	}
	if err != nil {
		return ClockOutResult{}, err
	}
	// Lewat batas, presensi masuk tetap terbuka tanpa jam kerja dan SDM yang mengoreksinya (D31).
	deadline, err := s.deadlineOf(ctx, open)
	if err != nil {
		return ClockOutResult{}, err
	}
	if !now.Before(deadline) {
		return ClockOutResult{}, notClockedIn()
	}
	// Izin dibaca dari work_date presensi masuknya, jadi presensi pulang lewat tengah malam tetap ikut hari itu.
	where, err := s.place(ctx, a, open.WorkDate.Time, parsed.point)
	if err != nil {
		return ClockOutResult{}, err
	}
	// Masuk dari kantor lalu pulang dari luar berarti hari itu tidak terbukti penuh di kantor.
	workMode := open.WorkMode
	if workMode == WorkModeOnsite {
		workMode = where.workMode
	}
	faces, err := s.checkFace(ctx, &a, in)
	if err != nil {
		return ClockOutResult{}, err
	}
	if err := s.assess(ctx, &a, parsed, now); err != nil {
		return ClockOutResult{}, err
	}
	// Sinyal saat pulang digabung dengan saat masuk; skor tertinggi yang disimpan.
	status := open.Status
	if a.score >= s.riskReviewMin() {
		status = store.AttStatusReview
	}
	flags, err := mergeFlags(open.RiskFlags, a.flags)
	if err != nil {
		return ClockOutResult{}, err
	}

	accepted, err := a.recorded(s.riskReviewMin())
	if err != nil {
		return ClockOutResult{}, err
	}
	worked := WorkedMinutes(open.ClockInAt.Time, now)
	var row store.Attendance
	closedMeanwhile := false
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		row, err = q.ClockOut(ctx, store.ClockOutParams{
			ID: open.ID, ClockOutAt: pgconv.Time(now), WorkedMinutes: pgtype.Int4{Int32: int32(worked), Valid: true},
			OutLatitude: coordinate(parsed.point.Latitude), OutLongitude: coordinate(parsed.point.Longitude),
			WorkMode: workMode, Status: status, RiskScore: max(open.RiskScore, int32(a.score)), RiskFlags: flags,
		})
		if pgconv.IsNoRows(err) {
			closedMeanwhile = true
			return nil
		}
		if err != nil {
			return err
		}
		if err := linkCapture(ctx, q, faces, row.ID); err != nil {
			return err
		}
		if err := q.InsertAttendanceAttempt(ctx, accepted); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AttendanceClockOut, Entity: "attendance", EntityID: row.ID, IP: ip,
		})
	})
	if err != nil {
		return ClockOutResult{}, err
	}
	if closedMeanwhile {
		return ClockOutResult{}, s.reject(ctx, a, "already_clocked_out", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_OUT", "Anda sudah presensi pulang."))
	}
	return ClockOutResult{
		AttendanceID: row.ID.String(), WorkDate: row.WorkDate.Time.Format("2006-01-02"),
		ClockOutAt: row.ClockOutAt.Time.UTC().Format(time.RFC3339), WorkedMinutes: worked,
		RequiresReview: row.Status == store.AttStatusReview,
	}, nil
}

// linkCapture mengaitkan selfie dengan presensi yang tercatat, untuk ditampilkan saat ditinjau.
func linkCapture(ctx context.Context, q *store.Queries, f faceResult, attendanceID uuid.UUID) error {
	if !f.capture.Valid {
		return nil
	}
	return q.LinkFaceCapture(ctx, store.LinkFaceCaptureParams{ID: f.capture.UUID, AttendanceID: uuid.NullUUID{UUID: attendanceID, Valid: true}})
}

func mergeFlags(raw []byte, more []string) ([]byte, error) {
	var flags []string
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &flags); err != nil {
			return nil, err
		}
	}
	for _, f := range more {
		if !slices.Contains(flags, f) {
			flags = append(flags, f)
		}
	}
	return json.Marshal(flags)
}

// Record adalah satu baris riwayat presensi sendiri.
type Record struct {
	ID            string  `json:"id"`
	WorkDate      string  `json:"work_date"`
	Status        string  `json:"status"`
	ClockInAt     *string `json:"clock_in_at"`
	ClockOutAt    *string `json:"clock_out_at"`
	LateMinutes   int32   `json:"late_minutes"`
	WorkedMinutes *int32  `json:"worked_minutes"`
	WorkMode      string  `json:"work_mode"`
	LocationName  *string `json:"location_name"`
}

// Mine mengembalikan riwayat presensi sendiri. Bawaannya 31 hari terakhir.
func (s *Service) Mine(ctx context.Context, p auth.Principal, fromRaw, toRaw string) ([]Record, error) {
	fields := map[string]string{}
	to := pgconv.WorkDate(s.opt.Now())
	if toRaw != "" {
		parsed, err := time.Parse("2006-01-02", toRaw)
		if err != nil {
			fields["to"] = "Tanggal tidak valid."
		}
		to = parsed
	}
	from := to.AddDate(0, 0, -30)
	if fromRaw != "" {
		parsed, err := time.Parse("2006-01-02", fromRaw)
		if err != nil {
			fields["from"] = "Tanggal tidak valid."
		}
		from = parsed
	}
	if len(fields) == 0 && (from.After(to) || to.Sub(from) > maxRangeDays*24*time.Hour) {
		fields["from"] = "Rentang tanggal maksimal 92 hari."
	}
	if len(fields) > 0 {
		return nil, httpx.Validation(fields)
	}
	rows, err := s.q.ListMyAttendance(ctx, store.ListMyAttendanceParams{
		EmployeeID: p.EmployeeID, FromDate: pgconv.Date(from), ToDate: pgconv.Date(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		record := Record{
			ID: r.ID.String(), WorkDate: r.WorkDate.Time.Format("2006-01-02"), Status: string(r.Status),
			ClockInAt: pgconv.TimeString(r.ClockInAt), ClockOutAt: pgconv.TimeString(r.ClockOutAt),
			LateMinutes: r.LateMinutes, WorkMode: r.WorkMode, LocationName: pgconv.TextPtr(r.LocationName),
		}
		if r.WorkedMinutes.Valid {
			record.WorkedMinutes = &r.WorkedMinutes.Int32
		}
		out = append(out, record)
	}
	return out, nil
}

// Handler melayani /v1/attendance.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

const headerDeviceID = "X-Device-Id"

func (h *Handler) Challenge(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Purpose string `json:"purpose"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.NewChallenge(r.Context(), p, r.Header.Get(headerDeviceID), in.Purpose)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

// readClock membaca presensi: multipart dengan bagian payload dan berkas selfie, atau JSON tanpa
// selfie yang hanya diterima di development (D36).
func readClock(r *http.Request) (ClockInput, error) {
	var in ClockInput
	files, err := readUploadFields(r, &in, map[string]int{"selfie": 1, "gesture": 1})
	if err != nil {
		return in, err
	}
	if f := files["selfie"]; len(f) == 1 {
		in.selfie = f[0]
	}
	if f := files["gesture"]; len(f) == 1 {
		in.gesture = f[0]
	}
	return in, nil
}

func (h *Handler) ClockIn(w http.ResponseWriter, r *http.Request) error {
	in, err := readClock(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.ClockIn(r.Context(), p, audit.ClientIP(r), r.Header.Get(headerDeviceID), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) ClockOut(w http.ResponseWriter, r *http.Request) error {
	in, err := readClock(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.ClockOut(r.Context(), p, audit.ClientIP(r), r.Header.Get(headerDeviceID), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := h.svc.Mine(r.Context(), p, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}
