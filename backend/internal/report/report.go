// Package report menyusun rekap presensi dan pratinjau ubar (QUERY.sql #8 dan #9).
package report

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
		Note:            "Rumus sementara: potongan alpha = ubar pokok / 26 per hari, potongan telat = Rp5.000 per kejadian. Menunggu keputusan SDM.",
	}
}

type ScopeResolver func(ctx context.Context, p auth.Principal, key string) (auth.ScopeFilter, error)

type Service struct {
	q     *store.Queries
	scope ScopeResolver
	now   func() time.Time
}

func NewService(q *store.Queries, scope ScopeResolver) *Service {
	return &Service{q: q, scope: scope, now: time.Now}
}

// LeaveTypeDays adalah jumlah hari cuti/izin satu jenis dalam periode.
type LeaveTypeDays struct {
	LeaveTypeID string `json:"leave_type_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	QuotaKind   string `json:"quota_kind"`
	Paid        bool   `json:"paid"`
	Days        int64  `json:"days"`
}

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
	// LeaveQuota memotong saldo (jenis statutory/custom), LeaveNonQuota tidak (jenis none), dan
	// LeaveOther adalah hari cuti tanpa pengajuan disetujui yang cocok. Ketiganya berjumlah Leave.
	LeaveQuota     int64           `json:"leave_quota"`
	LeaveNonQuota  int64           `json:"leave_non_quota"`
	LeaveOther     int64           `json:"leave_other"`
	LeaveUnpaid    int64           `json:"leave_unpaid"` // hari cuti dari jenis yang tidak dibayar (paid = false)
	LeaveByType    []LeaveTypeDays `json:"leave_by_type"`
	Absent         int64           `json:"absent"`
	Review         int64           `json:"review"` // presensi yang menunggu keputusan Review Presensi
	LateMinutes    int64           `json:"late_minutes"`
	WorkedMinutes  int64           `json:"worked_minutes"`
	MissedClockOut int64           `json:"missed_clock_out"`
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

// filter adalah scope dan rentang tanggal satu periode laporan.
type filter struct {
	from, to   time.Time
	all        bool
	subs, divs []uuid.UUID
}

func (s *Service) filterFor(ctx context.Context, p auth.Principal, period Period) (filter, error) {
	scope, err := s.scope(ctx, p, permission.AttendanceReport)
	if err != nil {
		return filter{}, err
	}
	f := filter{all: scope.All, subs: scope.SubdivisionIDs, divs: scope.DivisionIDs}
	if f.subs == nil {
		f.subs = []uuid.UUID{}
	}
	if f.divs == nil {
		f.divs = []uuid.UUID{}
	}
	f.from = time.Date(period.Year, period.Month, 1, 0, 0, 0, 0, time.UTC)
	f.to = f.from.AddDate(0, 1, 0)
	return f, nil
}

// wibWallClock mengubah waktu menjadi jam dinding WIB tanpa zona, untuk dibandingkan dengan tanggal kerja.
func wibWallClock(t time.Time) pgtype.Timestamp {
	w := t.In(pgconv.WIB)
	return pgtype.Timestamp{Time: time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, time.UTC), Valid: true}
}

func (s *Service) rows(ctx context.Context, period Period, f filter) ([]store.AttendanceReportRow, error) {
	return s.q.AttendanceReport(ctx, store.AttendanceReportParams{
		NowWib:              wibWallClock(s.now()),
		FromDate:            pgconv.Date(f.from),
		ToDate:              pgconv.Date(f.to),
		ScopeAll:            f.all,
		ScopeSubdivisionIds: f.subs,
		ScopeDivisionIds:    f.divs,
		DivisionID:          period.DivisionID,
		WorkMode:            period.WorkMode,
	})
}

func (s *Service) Attendance(ctx context.Context, p auth.Principal, period Period) ([]AttendanceRow, error) {
	f, err := s.filterFor(ctx, p, period)
	if err != nil {
		return nil, err
	}
	rows, err := s.rows(ctx, period, f)
	if err != nil {
		return nil, err
	}
	byEmployee := map[uuid.UUID][]store.AttendanceReportLeaveTypesRow{}
	// Dengan filter mode kerja, hari cuti tidak dihitung (tidak punya mode kerja).
	if period.WorkMode == "" {
		types, err := s.q.AttendanceReportLeaveTypes(ctx, store.AttendanceReportLeaveTypesParams{
			FromDate: pgconv.Date(f.from), ToDate: pgconv.Date(f.to), ScopeAll: f.all,
			ScopeSubdivisionIds: f.subs, ScopeDivisionIds: f.divs, DivisionID: period.DivisionID,
		})
		if err != nil {
			return nil, err
		}
		for _, t := range types {
			byEmployee[t.EmployeeID] = append(byEmployee[t.EmployeeID], t)
		}
	}
	out := make([]AttendanceRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, attendanceRow(r, byEmployee[r.EmployeeID]))
	}
	return out, nil
}

// attendanceRow menyusun satu baris rekap. Hari kerja mencakup presensi yang masih menunggu Review
// Presensi, karena hari itu tetap hari kerja apa pun keputusan SDM nanti.
func attendanceRow(r store.AttendanceReportRow, types []store.AttendanceReportLeaveTypesRow) AttendanceRow {
	out := AttendanceRow{
		EmployeeID: r.EmployeeID.String(), Nip: r.Nip, FullName: r.FullName,
		Division: r.Division, Subdivision: r.Subdivision,
		WorkDays: r.Present + r.Review + r.Leave + r.Absent,
		Present:  r.Present, Late: r.Late, Leave: r.Leave, Absent: r.Absent, Review: r.Review,
		LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, MissedClockOut: r.MissedClockOut,
		LeaveByType: make([]LeaveTypeDays, 0, len(types)),
	}
	matched := int64(0)
	for _, t := range types {
		out.LeaveByType = append(out.LeaveByType, LeaveTypeDays{
			LeaveTypeID: t.LeaveTypeID.String(), Code: t.Code, Name: t.Name, QuotaKind: string(t.QuotaKind), Paid: t.Paid, Days: t.Days,
		})
		if !t.Paid {
			out.LeaveUnpaid += t.Days
		}
		if t.QuotaKind == store.LeaveQuotaKindNone {
			out.LeaveNonQuota += t.Days
		} else {
			out.LeaveQuota += t.Days
		}
		matched += t.Days
	}
	out.LeaveOther = max(r.Leave-matched, 0)
	return out
}

func (s *Service) PayrollPreview(ctx context.Context, p auth.Principal, period Period) ([]PayrollRow, error) {
	f, err := s.filterFor(ctx, p, period)
	if err != nil {
		return nil, err
	}
	rows, err := s.rows(ctx, period, f)
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

// Calculate memakai decimal, tidak pernah float (MASTER.md §11.5). Ubar bersih minimum 0.
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
