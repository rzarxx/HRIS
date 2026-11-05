// Package shift mengelola shift kerja dan hari libur.
package shift

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const maxLateTolerance = 240

var clockPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Shift adalah objek baku respons. Jam dalam WIB.
type Shift struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	StartTime            string  `json:"start_time"`
	EndTime              string  `json:"end_time"`
	LateToleranceMinutes int32   `json:"late_tolerance_minutes"`
	WorkDays             []int16 `json:"work_days"`
}

// Holiday adalah objek baku hari libur.
type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

// Input juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya.
type Input struct {
	Name                 string  `json:"name"`
	StartTime            string  `json:"start_time"`
	EndTime              string  `json:"end_time"`
	LateToleranceMinutes *int32  `json:"late_tolerance_minutes"`
	WorkDays             []int16 `json:"work_days"`
}

type validated struct {
	Name      string
	StartTime pgtype.Time
	EndTime   pgtype.Time
	Tolerance int32
	WorkDays  []int16
}

func parseClock(s string) pgtype.Time {
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	return pgtype.Time{Microseconds: int64(h*3600+m*60) * 1_000_000, Valid: true}
}

func formatClock(t pgtype.Time) string {
	minutes := t.Microseconds / 60_000_000
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func (in Input) validate() (validated, error) {
	fields := map[string]string{}
	v := validated{Name: strings.TrimSpace(in.Name)}
	switch {
	case v.Name == "":
		fields["name"] = "Nama shift wajib diisi."
	case len(v.Name) > 80:
		fields["name"] = "Nama maksimal 80 karakter."
	}
	if !clockPattern.MatchString(in.StartTime) {
		fields["start_time"] = "Jam masuk harus berformat JJ:MM."
	} else {
		v.StartTime = parseClock(in.StartTime)
	}
	if !clockPattern.MatchString(in.EndTime) {
		fields["end_time"] = "Jam pulang harus berformat JJ:MM."
	} else {
		v.EndTime = parseClock(in.EndTime)
	}
	if fields["start_time"] == "" && fields["end_time"] == "" && in.StartTime == in.EndTime {
		fields["end_time"] = "Jam pulang tidak boleh sama dengan jam masuk."
	}
	if in.LateToleranceMinutes == nil || *in.LateToleranceMinutes < 0 || *in.LateToleranceMinutes > maxLateTolerance {
		fields["late_tolerance_minutes"] = "Toleransi telat harus 0 sampai 240 menit."
	} else {
		v.Tolerance = *in.LateToleranceMinutes
	}
	if len(in.WorkDays) == 0 {
		fields["work_days"] = "Pilih minimal satu hari kerja."
	}
	days := slices.Clone(in.WorkDays)
	slices.Sort(days)
	days = slices.Compact(days)
	for _, d := range days {
		if d < 1 || d > 7 {
			fields["work_days"] = "Hari kerja harus 1 (Senin) sampai 7 (Ahad)."
			break
		}
	}
	v.WorkDays = days
	if len(fields) > 0 {
		return v, httpx.Validation(fields)
	}
	return v, nil
}

// Service memuat aturan shift dan hari libur.
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: store.New(pool)} }

func toResponse(s store.Shift) Shift {
	return Shift{
		ID: s.ID.String(), Name: s.Name, StartTime: formatClock(s.StartTime), EndTime: formatClock(s.EndTime),
		LateToleranceMinutes: s.LateToleranceMinutes, WorkDays: s.WorkDays,
	}
}

func writeAudit(ctx context.Context, q *store.Queries, entry audit.Entry, entity string, id uuid.UUID, detail map[string]any) error {
	entry.Action, entry.Entity, entry.EntityID, entry.Detail = audit.SettingsChanged, entity, id, detail
	return audit.Write(ctx, q, entry)
}

func (s *Service) List(ctx context.Context) ([]Shift, error) {
	rows, err := s.q.ListShifts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Shift, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResponse(r))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, entry audit.Entry, in Input) (Shift, error) {
	v, err := in.validate()
	if err != nil {
		return Shift{}, err
	}
	var row store.Shift
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		row, err = q.InsertShift(ctx, store.InsertShiftParams{
			Name: v.Name, StartTime: v.StartTime, EndTime: v.EndTime, LateToleranceMinutes: v.Tolerance, WorkDays: v.WorkDays,
		})
		if err != nil {
			return err
		}
		return writeAudit(ctx, q, entry, "shift", row.ID, map[string]any{"change": "created"})
	})
	if err != nil {
		return Shift{}, err
	}
	return toResponse(row), nil
}

// Update memuat data lama lebih dulu, lalu menimpa dengan field yang dikirim klien.
func (s *Service) Update(ctx context.Context, entry audit.Entry, id uuid.UUID, decode func(*Input) error) (Shift, error) {
	current, err := s.q.GetShift(ctx, id)
	if pgconv.IsNoRows(err) {
		return Shift{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Shift{}, err
	}
	in := Input{
		Name: current.Name, StartTime: formatClock(current.StartTime), EndTime: formatClock(current.EndTime),
		LateToleranceMinutes: &current.LateToleranceMinutes, WorkDays: current.WorkDays,
	}
	if err := decode(&in); err != nil {
		return Shift{}, err
	}
	v, err := in.validate()
	if err != nil {
		return Shift{}, err
	}
	var row store.Shift
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		row, err = q.UpdateShift(ctx, store.UpdateShiftParams{
			ID: id, Name: v.Name, StartTime: v.StartTime, EndTime: v.EndTime, LateToleranceMinutes: v.Tolerance, WorkDays: v.WorkDays,
		})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		return writeAudit(ctx, q, entry, "shift", id, map[string]any{"change": "updated"})
	})
	if err != nil {
		return Shift{}, err
	}
	return toResponse(row), nil
}

func (s *Service) Delete(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.DeleteShift(ctx, id)
		if pgconv.IsForeignKeyViolation(err) {
			return httpx.NewError(http.StatusConflict, "CONFLICT_HAS_DEPENDENTS", "Shift sudah dipakai Pejuang atau presensi sehingga tidak bisa dihapus.")
		}
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return writeAudit(ctx, q, entry, "shift", id, map[string]any{"change": "deleted"})
	})
}

func (s *Service) ListHolidays(ctx context.Context, year int) ([]Holiday, error) {
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	rows, err := s.q.ListHolidays(ctx, store.ListHolidaysParams{FromDate: pgconv.Date(from), ToDate: pgconv.Date(from.AddDate(1, 0, 0))})
	if err != nil {
		return nil, err
	}
	out := make([]Holiday, 0, len(rows))
	for _, r := range rows {
		out = append(out, Holiday{Date: r.Date.Time.Format("2006-01-02"), Name: r.Name})
	}
	return out, nil
}

func (s *Service) CreateHoliday(ctx context.Context, entry audit.Entry, in Holiday) (Holiday, error) {
	fields := map[string]string{}
	date, err := pgconv.ParseDate(in.Date)
	if err != nil {
		fields["date"] = "Tanggal harus berformat TTTT-BB-HH."
	}
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		fields["name"] = "Nama hari libur wajib diisi."
	case len(name) > 120:
		fields["name"] = "Nama maksimal 120 karakter."
	}
	if len(fields) > 0 {
		return Holiday{}, httpx.Validation(fields)
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.InsertHoliday(ctx, store.InsertHolidayParams{Date: date, Name: name}); err != nil {
			if _, dup := pgconv.UniqueConstraint(err); dup {
				return httpx.Duplicate("date", "Tanggal ini sudah terdaftar sebagai hari libur.")
			}
			return err
		}
		return writeAudit(ctx, q, entry, "holiday", uuid.Nil, map[string]any{"change": "created", "date": in.Date})
	})
	if err != nil {
		return Holiday{}, err
	}
	return Holiday{Date: in.Date, Name: name}, nil
}

func (s *Service) DeleteHoliday(ctx context.Context, entry audit.Entry, raw string) error {
	date, err := pgconv.ParseDate(raw)
	if err != nil {
		return httpx.ErrNotFound()
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.DeleteHoliday(ctx, date)
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return writeAudit(ctx, q, entry, "holiday", uuid.Nil, map[string]any{"change": "deleted", "date": raw})
	})
}

// Handler melayani /v1/admin/shifts dan /v1/admin/holidays.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.Create(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.Update(r.Context(), actor(r), id, func(in *Input) error { return httpx.Decode(r, in) })
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), actor(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ListHolidays(w http.ResponseWriter, r *http.Request) error {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2000 || year > 2100 {
		return httpx.Validation(map[string]string{"year": "Tahun tidak valid."})
	}
	rows, err := h.svc.ListHolidays(r.Context(), year)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateHoliday(w http.ResponseWriter, r *http.Request) error {
	var in Holiday
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateHoliday(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) DeleteHoliday(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteHoliday(r.Context(), actor(r), chi.URLParam(r, "date")); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
