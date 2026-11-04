package attendance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	minAbsentNoteLen = 3
	maxReviewNoteLen = 500
)

// ReviewItem adalah satu absen di antrian Review. Skor dan alasan risiko hanya untuk peninjau (D8).
type ReviewItem struct {
	ID              string   `json:"id"`
	EmployeeID      string   `json:"employee_id"`
	EmployeeName    string   `json:"employee_name"`
	Nip             string   `json:"nip"`
	SubdivisionName string   `json:"subdivision_name"`
	DivisionName    string   `json:"division_name"`
	WorkDate        string   `json:"work_date"`
	ClockInAt       *string  `json:"clock_in_at"`
	ClockOutAt      *string  `json:"clock_out_at"`
	LateMinutes     int32    `json:"late_minutes"`
	WorkMode        string   `json:"work_mode"`
	FlexibleHours   bool     `json:"flexible_hours"`
	LocationName    *string  `json:"location_name"`
	InLatitude      *float64 `json:"in_latitude"`
	InLongitude     *float64 `json:"in_longitude"`
	OutLatitude     *float64 `json:"out_latitude"`
	OutLongitude    *float64 `json:"out_longitude"`
	InAccuracyM     *float64 `json:"in_accuracy_m"`
	RiskScore       int32    `json:"risk_score"`
	RiskFlags       []string `json:"risk_flags"`
}

func floatPtr(d decimal.NullDecimal) *float64 {
	if !d.Valid {
		return nil
	}
	v := d.Decimal.InexactFloat64()
	return &v
}

// ReviewQueue mengembalikan absen yang menunggu keputusan, yang terlama lebih dulu.
func (s *Service) ReviewQueue(ctx context.Context, workMode string, page httpx.Page) ([]ReviewItem, int64, error) {
	filter := pgtype.Text{}
	if workMode != "" {
		if workMode != WorkModeOnsite && workMode != WorkModeWFA && workMode != WorkModeField {
			return nil, 0, httpx.Validation(map[string]string{"work_mode": "Mode kerja tidak valid."})
		}
		filter = pgtype.Text{String: workMode, Valid: true}
	}
	total, err := s.q.CountAttendanceReview(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAttendanceReview(ctx, store.ListAttendanceReviewParams{WorkMode: filter, PageLimit: page.Limit(), PageOffset: page.Offset()})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ReviewItem, 0, len(rows))
	for _, r := range rows {
		flags := []string{}
		if len(r.RiskFlags) > 0 {
			if err := json.Unmarshal(r.RiskFlags, &flags); err != nil {
				return nil, 0, err
			}
		}
		out = append(out, ReviewItem{
			ID: r.ID.String(), EmployeeID: r.EmployeeID.String(), EmployeeName: r.EmployeeName, Nip: r.Nip,
			SubdivisionName: r.SubdivisionName, DivisionName: r.DivisionName, WorkDate: r.WorkDate.Time.Format("2006-01-02"),
			ClockInAt: pgconv.TimeString(r.ClockInAt), ClockOutAt: pgconv.TimeString(r.ClockOutAt), LateMinutes: r.LateMinutes,
			WorkMode: r.WorkMode, FlexibleHours: r.FlexibleHours, LocationName: pgconv.TextPtr(r.LocationName),
			InLatitude: floatPtr(r.InLatitude), InLongitude: floatPtr(r.InLongitude), OutLatitude: floatPtr(r.OutLatitude), OutLongitude: floatPtr(r.OutLongitude),
			InAccuracyM: floatPtr(r.InAccuracyM), RiskScore: r.RiskScore, RiskFlags: flags,
		})
	}
	return out, total, nil
}

// ResolveInput adalah keputusan peninjau.
type ResolveInput struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// ResolveResult adalah absen setelah diputuskan.
type ResolveResult struct {
	ID          string  `json:"id"`
	WorkDate    string  `json:"work_date"`
	Status      string  `json:"status"`
	LateMinutes int32   `json:"late_minutes"`
	ReviewNote  *string `json:"review_note"`
	ReviewedAt  string  `json:"reviewed_at"`
}

// Resolve memutuskan absen review (F11). Peninjau tidak boleh memutuskan absennya sendiri, karena
// keputusan ini memengaruhi upah.
func (s *Service) Resolve(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in ResolveInput) (ResolveResult, error) {
	note := strings.TrimSpace(in.Note)
	fields := map[string]string{}
	status := store.AttStatus(in.Decision)
	switch status {
	case store.AttStatusPresent, store.AttStatusLate:
	case store.AttStatusAbsent:
		if len(note) < minAbsentNoteLen {
			fields["note"] = "Catatan wajib diisi bila absen tidak valid, minimal 3 karakter."
		}
	default:
		fields["decision"] = "Pilih hadir, telat, atau tidak hadir."
	}
	if len(note) > maxReviewNoteLen {
		fields["note"] = "Catatan maksimal 500 karakter."
	}
	if len(fields) > 0 {
		return ResolveResult{}, httpx.Validation(fields)
	}

	var row store.Attendance
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		current, err := q.GetAttendanceForUpdate(ctx, id)
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		if current.EmployeeID == p.EmployeeID {
			return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Absen sendiri harus ditinjau peninjau lain.")
		}
		if current.Status != store.AttStatusReview {
			return httpx.NewError(http.StatusConflict, "REQUEST_NOT_PENDING", "Absen ini sudah ditinjau.")
		}
		late := int32(0)
		if status == store.AttStatusLate {
			if current.LateMinutes == 0 {
				return httpx.Validation(map[string]string{"decision": "Absen ini tidak terlambat. Pilih hadir."})
			}
			late = current.LateMinutes
		}
		row, err = q.ResolveAttendanceReview(ctx, store.ResolveAttendanceReviewParams{
			ID: id, Status: status, LateMinutes: late, ReviewBy: uuid.NullUUID{UUID: p.UserID, Valid: true},
			ReviewedAt: pgconv.Time(s.opt.Now()), ReviewNote: pgtype.Text{String: note, Valid: note != ""},
		})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AttendanceReviewResolved, Entity: "attendance", EntityID: id, IP: ip,
			Detail: map[string]any{"decision": in.Decision, "risk_score": current.RiskScore, "late_minutes_before": current.LateMinutes},
		})
	})
	if err != nil {
		return ResolveResult{}, err
	}
	return ResolveResult{
		ID: row.ID.String(), WorkDate: row.WorkDate.Time.Format("2006-01-02"), Status: string(row.Status), LateMinutes: row.LateMinutes,
		ReviewNote: pgconv.TextPtr(row.ReviewNote), ReviewedAt: row.ReviewedAt.Time.UTC().Format(time.RFC3339),
	}, nil
}

func (h *Handler) ReviewQueue(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	rows, total, err := h.svc.ReviewQueue(r.Context(), r.URL.Query().Get("work_mode"), page)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, page.Meta(total))
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in ResolveInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Resolve(r.Context(), p, audit.ClientIP(r), id, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
