package attendance

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// AttemptItem adalah satu upaya presensi, termasuk yang ditolak. Alasan dan skor hanya untuk peninjau (D8).
type AttemptItem struct {
	ID              int64    `json:"id"`
	EmployeeID      string   `json:"employee_id"`
	EmployeeName    string   `json:"employee_name"`
	Nip             string   `json:"nip"`
	SubdivisionName string   `json:"subdivision_name"`
	DivisionName    string   `json:"division_name"`
	Purpose         string   `json:"purpose"`
	Outcome         string   `json:"outcome"`
	ReasonCode      string   `json:"reason_code"`
	RiskScore       int32    `json:"risk_score"`
	RiskFlags       []string `json:"risk_flags"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	CreatedAt       string   `json:"created_at"`
}

const (
	OutcomeAccepted = "accepted"
	OutcomeReview   = "review"
	OutcomeRejected = "rejected"
)

// Attempts menampilkan upaya presensi pada satu tanggal kalender WIB, terbaru dulu. Tanggal kosong berarti hari ini.
func (s *Service) Attempts(ctx context.Context, dateRaw, outcome string, page httpx.Page) ([]AttemptItem, int64, error) {
	fields := map[string]string{}
	day := s.opt.Now().In(pgconv.WIB)
	if dateRaw != "" {
		parsed, err := pgconv.ParseDate(dateRaw)
		if err != nil {
			fields["date"] = "Tanggal tidak valid."
		}
		day = parsed.Time
	}
	filter := pgtype.Text{}
	if outcome != "" {
		if outcome != OutcomeAccepted && outcome != OutcomeReview && outcome != OutcomeRejected {
			fields["outcome"] = "Hasil tidak valid."
		}
		filter = pgtype.Text{String: outcome, Valid: true}
	}
	if len(fields) > 0 {
		return nil, 0, httpx.Validation(fields)
	}
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, pgconv.WIB)
	to := from.AddDate(0, 0, 1)
	total, err := s.q.CountAttendanceAttempts(ctx, store.CountAttendanceAttemptsParams{FromAt: pgconv.Time(from), ToAt: pgconv.Time(to), Outcome: filter})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAttendanceAttempts(ctx, store.ListAttendanceAttemptsParams{
		FromAt: pgconv.Time(from), ToAt: pgconv.Time(to), Outcome: filter, PageLimit: page.Limit(), PageOffset: page.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AttemptItem, 0, len(rows))
	for _, r := range rows {
		flags := []string{}
		if len(r.RiskFlags) > 0 {
			if err := json.Unmarshal(r.RiskFlags, &flags); err != nil {
				return nil, 0, err
			}
		}
		out = append(out, AttemptItem{
			ID: r.ID, EmployeeID: r.EmployeeID.String(), EmployeeName: r.EmployeeName, Nip: r.Nip,
			SubdivisionName: r.SubdivisionName, DivisionName: r.DivisionName, Purpose: r.Purpose, Outcome: r.Outcome,
			ReasonCode: r.ReasonCode, RiskScore: r.RiskScore, RiskFlags: flags,
			Latitude: floatPtr(r.Latitude), Longitude: floatPtr(r.Longitude), CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return out, total, nil
}

func (h *Handler) Attempts(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	rows, total, err := h.svc.Attempts(r.Context(), q.Get("date"), q.Get("outcome"), page)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, page.Meta(total))
}
