package attendance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
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
	// AllowUnverified mengizinkan absen tanpa bukti integritas perangkat dan wajah. Hanya untuk
	// development selama verifikasinya belum dibuat; bawaannya false agar semua absen ditolak (D36).
	AllowUnverified bool
	ChallengeTTL    time.Duration
	Now             func() time.Time
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
	return &Service{pool: pool, q: store.New(pool), opt: opt}
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

// ClockInput adalah isi absen masuk dan pulang.
type ClockInput struct {
	ChallengeID string   `json:"challenge_id"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	AccuracyM   *float64 `json:"accuracy_m"`
	CapturedAt  string   `json:"captured_at"`
	Signals     Signals  `json:"signals"`
}

type parsedClock struct {
	challengeID uuid.UUID
	point       Point
	accuracyM   float64
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
	if _, err := time.Parse(time.RFC3339, in.CapturedAt); err != nil {
		fields["captured_at"] = "Waktu pengambilan tidak valid."
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
}

// NewChallenge membuat tantangan sekali pakai yang terikat ke pengguna dan perangkat.
func (s *Service) NewChallenge(ctx context.Context, p auth.Principal, deviceHeader, purpose string) (Challenge, error) {
	if purpose != PurposeClockIn && purpose != PurposeClockOut {
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
	row, err := s.q.CreateAttendanceChallenge(ctx, store.CreateAttendanceChallengeParams{
		EmployeeID: p.EmployeeID, DeviceID: device.ID, Nonce: hex.EncodeToString(raw), Purpose: purpose,
		ExpiresAt: pgconv.Time(s.opt.Now().Add(s.opt.ChallengeTTL)),
	})
	if err != nil {
		return Challenge{}, err
	}
	return Challenge{ChallengeID: row.ID.String(), Nonce: row.Nonce, ExpiresAt: row.ExpiresAt.Time.UTC().Format(time.RFC3339)}, nil
}

// attempt mengumpulkan data satu upaya absen untuk attendance_attempts.
type attempt struct {
	p         auth.Principal
	ip        *netip.Addr
	purpose   string
	device    uuid.NullUUID
	challenge uuid.NullUUID
	point     Point
	flags     []string
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
		Outcome: outcome, ReasonCode: reason, RiskFlags: raw,
		Latitude: coordinate(a.point.Latitude), Longitude: coordinate(a.point.Longitude), Ip: a.ip,
	}, nil
}

// reject mencatat upaya yang ditolak lalu mengembalikan error untuk klien. Pencatatan berada di
// luar transaksi absen agar tetap tersimpan, dan alasan rinci hanya terlihat oleh SDM.
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

// verify menjalankan pemeriksaan yang sama untuk absen masuk dan pulang (F9), lalu mengembalikan
// lokasi kantor yang cocok.
func (s *Service) verify(ctx context.Context, a *attempt, deviceHeader string, in ClockInput, parsed parsedClock) (store.ListEmployeeLocationsRow, error) {
	var none store.ListEmployeeLocationsRow
	device, err := s.deviceOf(ctx, a.p, deviceHeader)
	if err != nil {
		if device.ID != uuid.Nil {
			a.device = uuid.NullUUID{UUID: device.ID, Valid: true}
		}
		return none, s.reject(ctx, *a, "device_not_approved", err)
	}
	a.device = uuid.NullUUID{UUID: device.ID, Valid: true}

	if _, err := s.q.ConsumeAttendanceChallenge(ctx, store.ConsumeAttendanceChallengeParams{
		Now: pgconv.Time(s.opt.Now()), ID: parsed.challengeID, EmployeeID: a.p.EmployeeID, DeviceID: device.ID, Purpose: a.purpose,
	}); err != nil {
		if pgconv.IsNoRows(err) {
			return none, s.reject(ctx, *a, "challenge_expired", httpx.NewError(http.StatusUnprocessableEntity, "CHALLENGE_EXPIRED", "Waktu absen habis, silakan ulangi."))
		}
		return none, err
	}
	a.challenge = uuid.NullUUID{UUID: parsed.challengeID, Valid: true}

	if !s.opt.AllowUnverified {
		return none, s.reject(ctx, *a, "integrity_failed", errBlocked())
	}
	a.flags = append(a.flags, flagUnverified)

	if hard := in.Signals.hardFlags(); len(hard) > 0 {
		a.flags = append(a.flags, hard...)
		return none, s.reject(ctx, *a, "hard_signal", errBlocked())
	}

	location, ok, err := s.matchLocation(ctx, a.p.EmployeeID, parsed.point)
	if err != nil {
		return none, err
	}
	if !ok {
		return none, s.reject(ctx, *a, "geofence_out", httpx.NewError(http.StatusUnprocessableEntity, "GEOFENCE_OUT", "Anda berada di luar area kantor."))
	}
	return location, nil
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

// ClockInResult adalah jawaban absen masuk. Skor dan alasan risiko tidak pernah dikirim ke klien.
type ClockInResult struct {
	AttendanceID   string      `json:"attendance_id"`
	WorkDate       string      `json:"work_date"`
	Status         string      `json:"status"`
	ClockInAt      string      `json:"clock_in_at"`
	LateMinutes    int         `json:"late_minutes"`
	OfficeLocation LocationRef `json:"office_location"`
	RequiresReview bool        `json:"requires_review"`
}

// ClockIn mencatat absen masuk.
func (s *Service) ClockIn(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in ClockInput) (ClockInResult, error) {
	parsed, err := in.parse()
	if err != nil {
		return ClockInResult{}, err
	}
	a := attempt{p: p, ip: ip, purpose: PurposeClockIn, point: parsed.point}
	location, err := s.verify(ctx, &a, deviceHeader, in, parsed)
	if err != nil {
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
	holiday, err := s.q.IsHoliday(ctx, pgconv.Date(workDate))
	if err != nil {
		return ClockInResult{}, err
	}
	status := store.AttStatusPresent
	late := 0
	if holiday || !IsWorkDay(workDate, schedule) {
		a.flags = append(a.flags, flagOffSchedule)
	} else if late = LateMinutes(now, workDate, schedule); late > 0 {
		status = store.AttStatusLate
	}

	accepted, err := a.params("accepted", "ok")
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
			RiskFlags:   accepted.RiskFlags, Status: status, LateMinutes: int32(late),
			LocationID: uuid.NullUUID{UUID: location.ID, Valid: true},
			InLatitude: coordinate(parsed.point.Latitude), InLongitude: coordinate(parsed.point.Longitude),
		})
		if pgconv.IsNoRows(err) {
			conflict = true
			return nil
		}
		if err != nil {
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
		return ClockInResult{}, s.reject(ctx, a, "already_clocked_in", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_IN", "Anda sudah absen masuk."))
	}
	return ClockInResult{
		AttendanceID: row.ID.String(), WorkDate: workDate.Format("2006-01-02"), Status: string(row.Status),
		ClockInAt: row.ClockInAt.Time.UTC().Format(time.RFC3339), LateMinutes: late,
		OfficeLocation: LocationRef{ID: location.ID.String(), Name: location.Name}, RequiresReview: false,
	}, nil
}

// ClockOutResult adalah jawaban absen pulang.
type ClockOutResult struct {
	AttendanceID  string `json:"attendance_id"`
	WorkDate      string `json:"work_date"`
	ClockOutAt    string `json:"clock_out_at"`
	WorkedMinutes int    `json:"worked_minutes"`
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

// ClockOut mencatat absen pulang pada absen masuk yang masih terbuka (F10).
func (s *Service) ClockOut(ctx context.Context, p auth.Principal, ip *netip.Addr, deviceHeader string, in ClockInput) (ClockOutResult, error) {
	parsed, err := in.parse()
	if err != nil {
		return ClockOutResult{}, err
	}
	a := attempt{p: p, ip: ip, purpose: PurposeClockOut, point: parsed.point}
	if _, err := s.verify(ctx, &a, deviceHeader, in, parsed); err != nil {
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
				return s.reject(ctx, a, "already_clocked_out", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_OUT", "Anda sudah absen pulang."))
			}
		} else if !pgconv.IsNoRows(err) {
			return err
		}
		return s.reject(ctx, a, "not_clocked_in", httpx.NewError(http.StatusConflict, "NOT_CLOCKED_IN", "Anda belum absen masuk."))
	}

	open, err := s.q.GetOpenAttendance(ctx, p.EmployeeID)
	if pgconv.IsNoRows(err) {
		return ClockOutResult{}, notClockedIn()
	}
	if err != nil {
		return ClockOutResult{}, err
	}
	// Lewat batas, absen masuk tetap terbuka tanpa jam kerja dan SDM yang mengoreksinya (D31).
	deadline, err := s.deadlineOf(ctx, open)
	if err != nil {
		return ClockOutResult{}, err
	}
	if !now.Before(deadline) {
		return ClockOutResult{}, notClockedIn()
	}

	accepted, err := a.params("accepted", "ok")
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
		})
		if pgconv.IsNoRows(err) {
			closedMeanwhile = true
			return nil
		}
		if err != nil {
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
		return ClockOutResult{}, s.reject(ctx, a, "already_clocked_out", httpx.NewError(http.StatusConflict, "ALREADY_CLOCKED_OUT", "Anda sudah absen pulang."))
	}
	return ClockOutResult{
		AttendanceID: row.ID.String(), WorkDate: row.WorkDate.Time.Format("2006-01-02"),
		ClockOutAt: row.ClockOutAt.Time.UTC().Format(time.RFC3339), WorkedMinutes: worked,
	}, nil
}

// Record adalah satu baris riwayat absen sendiri.
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

// Mine mengembalikan riwayat absen sendiri. Bawaannya 31 hari terakhir.
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

func (h *Handler) ClockIn(w http.ResponseWriter, r *http.Request) error {
	var in ClockInput
	if err := httpx.Decode(r, &in); err != nil {
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
	var in ClockInput
	if err := httpx.Decode(r, &in); err != nil {
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
