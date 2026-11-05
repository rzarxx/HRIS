// Package report menyusun rekap presensi dan pratinjau upah (QUERY.sql #8 dan #9).
package report

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Rumus sementara, menunggu keputusan SDM (MASTER.md §14.2 no. 2).
const (
	rulesVersion    = "preview-v0"
	workDaysDivisor = 26
)

var latePenalty = decimal.NewFromInt(5000)

// Rules ditampilkan ke pengguna agar jelas rumus yang dipakai.
type Rules struct {
	Version         string `json:"version"`
	WorkDaysDivisor int    `json:"work_days_divisor"`
	LatePenalty     string `json:"late_penalty"`
	Note            string `json:"note"`
}

func currentRules() Rules {
	return Rules{
		Version:         rulesVersion,
		WorkDaysDivisor: workDaysDivisor,
		LatePenalty:     latePenalty.StringFixed(2),
		Note:            "Rumus sementara: potongan alpha = gaji pokok / 26 per hari, potongan telat = Rp5.000 per kejadian. Menunggu keputusan SDM.",
	}
}

type ScopeResolver func(ctx context.Context, p auth.Principal, key string) (auth.ScopeFilter, error)

type Service struct {
	q     *store.Queries
	scope ScopeResolver
}

func NewService(q *store.Queries, scope ScopeResolver) *Service { return &Service{q: q, scope: scope} }

type AttendanceRow struct {
	EmployeeID  string `json:"employee_id"`
	Nip         string `json:"nip"`
	FullName    string `json:"full_name"`
	Division    string `json:"division"`
	Subdivision string `json:"subdivision"`
	WorkDays    int64  `json:"work_days"`
	Present     int64  `json:"present"`
	Late        int64  `json:"late"`
	Leave       int64  `json:"leave"`
	Absent      int64  `json:"absent"`
	LateMinutes int64  `json:"late_minutes"`
}

type PayrollRow struct {
	EmployeeID      string `json:"employee_id"`
	Nip             string `json:"nip"`
	FullName        string `json:"full_name"`
	Division        string `json:"division"`
	Subdivision     string `json:"subdivision"`
	BaseSalary      string `json:"base_salary"`
	Absent          int64  `json:"absent"`
	Late            int64  `json:"late"`
	AbsentDeduction string `json:"absent_deduction"`
	LateDeduction   string `json:"late_deduction"`
	Net             string `json:"net"`
}

// Period adalah bulan laporan.
type Period struct {
	Year       int
	Month      time.Month
	DivisionID uuid.NullUUID
	WorkMode   string
}

func (s *Service) rows(ctx context.Context, p auth.Principal, period Period) ([]store.AttendanceReportRow, error) {
	scope, err := s.scope(ctx, p, permission.AttendanceReport)
	if err != nil {
		return nil, err
	}
	subs, divs := scope.SubdivisionIDs, scope.DivisionIDs
	if subs == nil {
		subs = []uuid.UUID{}
	}
	if divs == nil {
		divs = []uuid.UUID{}
	}
	from := time.Date(period.Year, period.Month, 1, 0, 0, 0, 0, time.UTC)
	return s.q.AttendanceReport(ctx, store.AttendanceReportParams{
		FromDate:            pgconv.Date(from),
		ToDate:              pgconv.Date(from.AddDate(0, 1, 0)),
		ScopeAll:            scope.All,
		ScopeSubdivisionIds: subs,
		ScopeDivisionIds:    divs,
		DivisionID:          period.DivisionID,
		WorkMode:            period.WorkMode,
	})
}

func (s *Service) Attendance(ctx context.Context, p auth.Principal, period Period) ([]AttendanceRow, error) {
	rows, err := s.rows(ctx, p, period)
	if err != nil {
		return nil, err
	}
	out := make([]AttendanceRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AttendanceRow{
			EmployeeID: r.EmployeeID.String(), Nip: r.Nip, FullName: r.FullName,
			Division: r.Division, Subdivision: r.Subdivision,
			WorkDays: r.Present + r.Leave + r.Absent,
			Present:  r.Present, Late: r.Late, Leave: r.Leave, Absent: r.Absent, LateMinutes: r.LateMinutes,
		})
	}
	return out, nil
}

func (s *Service) PayrollPreview(ctx context.Context, p auth.Principal, period Period) ([]PayrollRow, error) {
	rows, err := s.rows(ctx, p, period)
	if err != nil {
		return nil, err
	}
	out := make([]PayrollRow, 0, len(rows))
	for _, r := range rows {
		absentDeduction, lateDeduction, net := Calculate(r.BaseSalary, r.Absent, r.Late)
		out = append(out, PayrollRow{
			EmployeeID: r.EmployeeID.String(), Nip: r.Nip, FullName: r.FullName,
			Division: r.Division, Subdivision: r.Subdivision,
			BaseSalary: r.BaseSalary.StringFixed(2), Absent: r.Absent, Late: r.Late,
			AbsentDeduction: absentDeduction.StringFixed(2),
			LateDeduction:   lateDeduction.StringFixed(2),
			Net:             net.StringFixed(2),
		})
	}
	return out, nil
}

// Calculate memakai decimal, tidak pernah float (MASTER.md §11.5). Upah bersih minimum 0.
func Calculate(base decimal.Decimal, absent, late int64) (absentDeduction, lateDeduction, net decimal.Decimal) {
	absentDeduction = base.Mul(decimal.NewFromInt(absent)).Div(decimal.NewFromInt(workDaysDivisor)).Round(0)
	lateDeduction = latePenalty.Mul(decimal.NewFromInt(late))
	net = base.Sub(absentDeduction).Sub(lateDeduction)
	if net.IsNegative() {
		net = decimal.Zero
	}
	return absentDeduction, lateDeduction, net
}

// Handler melayani /v1/admin/reports/*.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func readPeriod(r *http.Request) (Period, error) {
	fields := map[string]string{}
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2000 || year > 2100 {
		fields["year"] = "Tahun tidak valid."
	}
	month, err := strconv.Atoi(r.URL.Query().Get("month"))
	if err != nil || month < 1 || month > 12 {
		fields["month"] = "Bulan tidak valid."
	}
	if len(fields) > 0 {
		return Period{}, httpx.Validation(fields)
	}
	divisionID, err := httpx.QueryUUID(r, "division_id")
	if err != nil {
		return Period{}, err
	}
	return Period{Year: year, Month: time.Month(month), DivisionID: divisionID}, nil
}

func (h *Handler) Attendance(w http.ResponseWriter, r *http.Request) error {
	period, err := readPeriod(r)
	if err != nil {
		return err
	}
	period.WorkMode = r.URL.Query().Get("work_mode")
	if period.WorkMode != "" && period.WorkMode != "onsite" && period.WorkMode != "wfa" && period.WorkMode != "field" {
		return httpx.Validation(map[string]string{"work_mode": "Mode kerja tidak valid."})
	}
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := h.svc.Attendance(r.Context(), p, period)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) PayrollPreview(w http.ResponseWriter, r *http.Request) error {
	period, err := readPeriod(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := h.svc.PayrollPreview(r.Context(), p, period)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, map[string]any{"rules": currentRules()})
}
