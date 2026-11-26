package payroll

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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

// Status periode; nilainya sama dengan enum payroll_status.
const (
	StatusDraft           = "draft"
	StatusPendingApproval = "pending_approval"
	StatusFinal           = "final"
	StatusPaid            = "paid"
)

const (
	minRejectNote = 3
	maxRejectNote = 500
	maxInputNote  = 300
	maxQuantity   = 10_000
)

// PeriodRange menghitung rentang periode berlabel bulan year-month dari tanggal cutoff (D31): cutoff 25 berarti
// 26 bulan sebelumnya sampai 25 bulan ini; cutoff 0 berarti bulan kalender. end inklusif.
func PeriodRange(year int, month time.Month, cutoffDay int) (start, end time.Time) {
	if cutoffDay <= 0 || cutoffDay > 28 {
		start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, -1)
	}
	return time.Date(year, month-1, cutoffDay+1, 0, 0, 0, 0, time.UTC), time.Date(year, month, cutoffDay, 0, 0, 0, 0, time.UTC)
}

// RangeFor membaca setelan cutoff lalu menghitung rentang periode; dipakai juga rekap presensi.
func (s *Service) RangeFor(ctx context.Context, year int, month time.Month) (start, end time.Time, err error) {
	set, err := s.Settings(ctx)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, end = PeriodRange(year, month, set.PeriodCutoffDay)
	return start, end, nil
}

// Snapshot adalah salinan komponen dan setelan yang dipakai saat generate.
type Snapshot struct {
	Components []Component `json:"components"`
	MinNetZero bool        `json:"min_net_zero"`
	CutoffDay  int         `json:"period_cutoff_day"`
}

// AttendanceSummary disimpan per slip; Review dan MissedClockOut dicatat agar terlihat bila ada yang belum beres.
type AttendanceSummary struct {
	Attendance
	Review         int64 `json:"review"`
	MissedClockOut int64 `json:"missed_clock_out"`
}

// PeriodView adalah respons API periode ubar.
type PeriodView struct {
	ID            string  `json:"id"`
	Year          int     `json:"year"`
	Month         int     `json:"month"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Status        string  `json:"status"`
	EmployeeCount int64   `json:"employee_count"`
	TotalNet      string  `json:"total_net"`
	GeneratedAt   *string `json:"generated_at"`
	SubmittedAt   *string `json:"submitted_at"`
	FinalizedAt   *string `json:"finalized_at"`
	PaidAt        *string `json:"paid_at"`
	RejectNote    *string `json:"reject_note"`
	// CanFinalize bernilai true bila pemanggil bukan pembuat (generate atau ajukan) periode ini.
	CanFinalize bool `json:"can_finalize"`
}

func periodView(p store.PayrollPeriod, viewer uuid.UUID, count int64, total decimal.Decimal) PeriodView {
	return PeriodView{
		ID: p.ID.String(), Year: int(p.Year), Month: int(p.Month),
		StartDate: p.StartDate.Time.Format(time.DateOnly), EndDate: p.EndDate.Time.Format(time.DateOnly),
		Status: string(p.Status), EmployeeCount: count, TotalNet: total.StringFixed(2),
		GeneratedAt: pgconv.TimeString(p.GeneratedAt), SubmittedAt: pgconv.TimeString(p.SubmittedAt),
		FinalizedAt: pgconv.TimeString(p.FinalizedAt), PaidAt: pgconv.TimeString(p.PaidAt),
		RejectNote:  pgconv.TextPtr(p.RejectNote),
		CanFinalize: !isMaker(p, viewer),
	}
}

func isMaker(p store.PayrollPeriod, user uuid.UUID) bool {
	return (p.GeneratedBy.Valid && p.GeneratedBy.UUID == user) || (p.SubmittedBy.Valid && p.SubmittedBy.UUID == user)
}

func statusError(message string) error {
	return httpx.NewError(http.StatusConflict, "PERIOD_LOCKED", message)
}

func (s *Service) ListPeriods(ctx context.Context, viewer uuid.UUID) ([]PeriodView, error) {
	rows, err := s.q.ListPayrollPeriods(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PeriodView, 0, len(rows))
	for _, r := range rows {
		p := store.PayrollPeriod{
			ID: r.ID, Year: r.Year, Month: r.Month, StartDate: r.StartDate, EndDate: r.EndDate, Status: r.Status,
			CreatedBy: r.CreatedBy, GeneratedBy: r.GeneratedBy, GeneratedAt: r.GeneratedAt, SubmittedBy: r.SubmittedBy,
			SubmittedAt: r.SubmittedAt, FinalizedAt: r.FinalizedAt, PaidAt: r.PaidAt, RejectNote: r.RejectNote,
		}
		out = append(out, periodView(p, viewer, r.EmployeeCount, r.TotalNet))
	}
	return out, nil
}

// PeriodInput membuat periode baru.
type PeriodInput struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

// CreatePeriod membuat periode draft. Periode bersambung tanpa celah: bila sudah ada periode sebelumnya, periode
// baru mulai sehari setelah periode terakhir berakhir.
func (s *Service) CreatePeriod(ctx context.Context, entry audit.Entry, in PeriodInput) (PeriodView, error) {
	if in.Year < 2000 || in.Year > 2100 || in.Month < 1 || in.Month > 12 {
		return PeriodView{}, httpx.Validation(map[string]string{"month": "Mohon pilih bulan dan tahun yang sesuai."})
	}
	start, end, err := s.RangeFor(ctx, in.Year, time.Month(in.Month))
	if err != nil {
		return PeriodView{}, err
	}
	var row store.PayrollPeriod
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		latest, err := q.LatestPayrollPeriod(ctx)
		switch {
		case pgconv.IsNoRows(err):
		case err != nil:
			return err
		default:
			if in.Year*12+in.Month <= int(latest.Year)*12+int(latest.Month) {
				return httpx.Validation(map[string]string{"month": "Periode harus setelah periode terakhir."})
			}
			start = latest.EndDate.Time.AddDate(0, 0, 1)
		}
		if end.Before(start) || end.Sub(start) > 61*24*time.Hour {
			return httpx.Validation(map[string]string{"month": "Rentang periode tidak valid. Periksa tanggal cutoff dan periode terakhir."})
		}
		overlap, err := q.PayrollPeriodOverlaps(ctx, store.PayrollPeriodOverlapsParams{StartDate: pgconv.Date(start), EndDate: pgconv.Date(end)})
		if err != nil {
			return err
		}
		if overlap {
			return httpx.Validation(map[string]string{"month": "Rentang periode bertumpuk dengan periode lain."})
		}
		row, err = q.InsertPayrollPeriod(ctx, store.InsertPayrollPeriodParams{
			Year: int16(in.Year), Month: int16(in.Month), StartDate: pgconv.Date(start), EndDate: pgconv.Date(end), CreatedBy: entry.UserID.UUID,
		})
		if _, dup := pgconv.UniqueConstraint(err); dup {
			return httpx.Validation(map[string]string{"month": "Periode bulan ini sudah ada."})
		}
		if err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollPeriodCreated, "payroll_period", row.ID
		entry.Detail = map[string]any{"year": in.Year, "month": in.Month, "start_date": start.Format(time.DateOnly), "end_date": end.Format(time.DateOnly)}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return PeriodView{}, err
	}
	return periodView(row, entry.UserID.UUID, 0, decimal.Zero), nil
}

// DeletePeriod menghapus periode yang masih draft beserta hasil hitungnya.
func (s *Service) DeletePeriod(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := q.LockPayrollPeriod(ctx, id)
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		if string(p.Status) != StatusDraft {
			return statusError("Hanya periode draft yang bisa dihapus.")
		}
		if err := q.DeletePayrollPeriod(ctx, id); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollPeriodCreated, "payroll_period", id
		entry.Detail = map[string]any{"change": "deleted", "year": p.Year, "month": p.Month}
		return audit.Write(ctx, q, entry)
	})
}

func lockPeriod(ctx context.Context, q *store.Queries, id uuid.UUID) (store.PayrollPeriod, error) {
	p, err := q.LockPayrollPeriod(ctx, id)
	if pgconv.IsNoRows(err) {
		return p, httpx.ErrNotFound()
	}
	return p, err
}

// attendanceRows mengambil ringkasan presensi semua Pejuang dalam rentang periode (scope penuh, khusus SDM).
func (s *Service) attendanceRows(ctx context.Context, q *store.Queries, p store.PayrollPeriod) (map[uuid.UUID]AttendanceSummary, []uuid.UUID, error) {
	from, to := pgconv.Date(p.StartDate.Time), pgconv.Date(p.EndDate.Time.AddDate(0, 0, 1))
	rows, err := q.AttendanceReport(ctx, store.AttendanceReportParams{
		NowWib: wibWallClock(s.now()), FromDate: from, ToDate: to, ScopeAll: true,
		ScopeSubdivisionIds: []uuid.UUID{}, ScopeDivisionIds: []uuid.UUID{},
	})
	if err != nil {
		return nil, nil, err
	}
	types, err := q.AttendanceReportLeaveTypes(ctx, store.AttendanceReportLeaveTypesParams{
		FromDate: from, ToDate: to, ScopeAll: true, ScopeSubdivisionIds: []uuid.UUID{}, ScopeDivisionIds: []uuid.UUID{},
	})
	if err != nil {
		return nil, nil, err
	}
	unpaid := map[uuid.UUID]int64{}
	for _, t := range types {
		if !t.Paid {
			unpaid[t.EmployeeID] += t.Days
		}
	}
	out := make(map[uuid.UUID]AttendanceSummary, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out[r.EmployeeID] = AttendanceSummary{
			Attendance: Attendance{
				WorkDays: decimal.NewFromFloat(r.WorkDays), PresentDays: r.Present, LateCount: r.Late, LateMinutes: r.LateMinutes,
				AbsentDays: r.Absent, LeaveDays: r.Leave, UnpaidLeaveDays: unpaid[r.EmployeeID],
			},
			Review: r.Review, MissedClockOut: r.MissedClockOut,
		}
		ids = append(ids, r.EmployeeID)
	}
	return out, ids, nil
}

// wibWallClock mengubah waktu menjadi jam dinding WIB tanpa zona (sama dengan rekap presensi).
func wibWallClock(t time.Time) pgtype.Timestamp {
	w := t.In(pgconv.WIB)
	return pgtype.Timestamp{Time: time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, time.UTC), Valid: true}
}

func inputsByEmployee(rows []store.PayrollInput) map[uuid.UUID]map[string]Input {
	out := map[uuid.UUID]map[string]Input{}
	for _, r := range rows {
		if out[r.EmployeeID] == nil {
			out[r.EmployeeID] = map[string]Input{}
		}
		out[r.EmployeeID][r.ComponentID.String()] = Input{Quantity: r.Quantity, Amount: r.Amount}
	}
	return out
}

type payrollSource struct {
	info       store.PayrollEmployeeInfoRow
	attendance AttendanceSummary
	values     map[string]decimal.Decimal
	inputs     map[string]Input
}

// writePayroll menghitung dan menyimpan slip satu Pejuang dalam periode.
func writePayroll(ctx context.Context, q *store.Queries, periodID uuid.UUID, snap Snapshot, src payrollSource) error {
	res := Calculate(src.info.BaseSalary, src.attendance.Attendance, snap.Components, src.values, src.inputs, snap.MinNetZero)
	att, err := json.Marshal(src.attendance)
	if err != nil {
		return err
	}
	values, err := json.Marshal(src.values)
	if err != nil {
		return err
	}
	id, err := q.InsertPayroll(ctx, store.InsertPayrollParams{
		PeriodID: periodID, EmployeeID: src.info.ID, Nip: src.info.Nip, FullName: src.info.FullName,
		Division: src.info.Division, Subdivision: src.info.Subdivision, Position: src.info.Position,
		BaseSalary: res.Base, Gross: res.Gross, Deductions: res.Deductions, Net: res.Net, NetClamped: res.NetClamped,
		Attendance: att, EmployeeValues: values,
	})
	if err != nil {
		return err
	}
	for i, it := range res.Items {
		if err := q.InsertPayrollItem(ctx, store.InsertPayrollItemParams{
			PayrollID: id, Position: int16(i + 1), ComponentCode: it.Code, Name: it.Name, Kind: store.PayrollComponentKind(it.Kind),
			Quantity: it.Quantity, Rate: it.Rate, Amount: it.Amount, ShowOnSlip: it.ShowOnSlip, Manual: it.Manual,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Generate menghitung ulang seluruh slip periode draft dari presensi, komponen aktif, nilai per Pejuang, dan
// isian periode. Aman diulang; isian SDM tidak hilang.
func (s *Service) Generate(ctx context.Context, entry audit.Entry, id uuid.UUID) (PeriodView, error) {
	var view PeriodView
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, id)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusDraft {
			return statusError("Periode yang sudah diajukan atau final tidak bisa dihitung ulang.")
		}
		set, err := s.Settings(ctx)
		if err != nil {
			return err
		}
		comps, err := q.ListActivePayrollComponents(ctx)
		if err != nil {
			return err
		}
		snap := Snapshot{Components: make([]Component, 0, len(comps)), MinNetZero: set.MinNetZero, CutoffDay: set.PeriodCutoffDay}
		for _, c := range comps {
			snap.Components = append(snap.Components, ToComponent(c))
		}
		attendance, ids, err := s.attendanceRows(ctx, q, p)
		if err != nil {
			return err
		}
		infos, err := q.PayrollEmployeeInfo(ctx, ids)
		if err != nil {
			return err
		}
		valueRows, err := q.ListAllEmployeePayrollComponents(ctx)
		if err != nil {
			return err
		}
		values := map[uuid.UUID]map[string]decimal.Decimal{}
		for _, v := range valueRows {
			if values[v.EmployeeID] == nil {
				values[v.EmployeeID] = map[string]decimal.Decimal{}
			}
			values[v.EmployeeID][v.ComponentID.String()] = v.Amount
		}
		inputRows, err := q.ListPayrollInputsOfPeriod(ctx, id)
		if err != nil {
			return err
		}
		inputs := inputsByEmployee(inputRows)
		if err := q.DeletePayrollsOfPeriod(ctx, id); err != nil {
			return err
		}
		total := decimal.Zero
		for _, info := range infos {
			v := values[info.ID]
			if v == nil {
				v = map[string]decimal.Decimal{}
			}
			if err := writePayroll(ctx, q, id, snap, payrollSource{info: info, attendance: attendance[info.ID], values: v, inputs: inputs[info.ID]}); err != nil {
				return err
			}
		}
		rows, err := q.ListPayrollsOfPeriod(ctx, id)
		if err != nil {
			return err
		}
		for _, r := range rows {
			total = total.Add(r.Net)
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		now := s.now()
		if err := q.SetPayrollPeriodGenerated(ctx, store.SetPayrollPeriodGeneratedParams{
			ID: id, RulesSnapshot: raw, GeneratedBy: entry.UserID, GeneratedAt: pgconv.Time(now),
		}); err != nil {
			return err
		}
		p.GeneratedBy, p.GeneratedAt, p.RejectNote = entry.UserID, pgconv.Time(now), pgtype.Text{}
		view = periodView(p, entry.UserID.UUID, int64(len(rows)), total)
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollGenerated, "payroll_period", id
		entry.Detail = map[string]any{"employees": len(rows), "components": len(snap.Components), "total_net": total.StringFixed(2)}
		return audit.Write(ctx, q, entry)
	})
	return view, err
}

// Readiness menghitung presensi yang belum beres dalam rentang periode; periode baru bisa diajukan bila nol.
type Readiness struct {
	Review         int64 `json:"review"`
	MissedClockOut int64 `json:"missed_clock_out"`
}

func (s *Service) readiness(ctx context.Context, q *store.Queries, p store.PayrollPeriod) (Readiness, error) {
	rows, _, err := s.attendanceRows(ctx, q, p)
	if err != nil {
		return Readiness{}, err
	}
	var r Readiness
	for _, a := range rows {
		r.Review += a.Review
		r.MissedClockOut += a.MissedClockOut
	}
	return r, nil
}

// Submit mengajukan periode untuk disetujui pemeriksa. Presensi dalam rentang lalu terkunci (D67).
func (s *Service) Submit(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, id)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusDraft {
			return statusError("Periode ini sudah diajukan.")
		}
		if !p.GeneratedAt.Valid {
			return httpx.NewError(http.StatusUnprocessableEntity, "PAYROLL_NOT_READY", "Hitung ubar periode ini lebih dulu.")
		}
		r, err := s.readiness(ctx, q, p)
		if err != nil {
			return err
		}
		if r.Review > 0 || r.MissedClockOut > 0 {
			e := httpx.NewError(http.StatusUnprocessableEntity, "PAYROLL_NOT_READY",
				"Masih ada presensi yang menunggu review atau lupa absen pulang di periode ini. Selesaikan dulu, lalu hitung ulang.")
			e.Details = map[string]any{"review": r.Review, "missed_clock_out": r.MissedClockOut}
			return e
		}
		if err := q.SetPayrollPeriodSubmitted(ctx, store.SetPayrollPeriodSubmittedParams{ID: id, SubmittedBy: entry.UserID, SubmittedAt: pgconv.Time(s.now())}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollSubmitted, "payroll_period", id
		return audit.Write(ctx, q, entry)
	})
}

// RejectInput mengembalikan periode ke draft.
type RejectInput struct {
	Note string `json:"note"`
}

func (s *Service) Reject(ctx context.Context, entry audit.Entry, id uuid.UUID, in RejectInput) error {
	note := cleanText(in.Note)
	if n := utf8.RuneCountInString(note); n < minRejectNote || n > maxRejectNote {
		return httpx.Validation(map[string]string{"note": "Alasan diisi 3 sampai 500 karakter."})
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, id)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusPendingApproval {
			return statusError("Hanya periode yang menunggu persetujuan yang bisa ditolak.")
		}
		if err := q.SetPayrollPeriodRejected(ctx, store.SetPayrollPeriodRejectedParams{
			ID: id, RejectNote: pgtype.Text{String: note, Valid: true}, RejectedBy: entry.UserID, RejectedAt: pgconv.Time(s.now()),
		}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID, entry.Detail = audit.PayrollRejected, "payroll_period", id, map[string]any{"note": note}
		return audit.Write(ctx, q, entry)
	})
}

// Finalize menyetujui periode. Pemeriksa wajib orang lain dari yang menghitung atau mengajukan (maker-checker).
func (s *Service) Finalize(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, id)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusPendingApproval {
			return statusError("Hanya periode yang menunggu persetujuan yang bisa difinalkan.")
		}
		if isMaker(p, entry.UserID.UUID) {
			return httpx.NewError(http.StatusForbidden, "MAKER_CHECKER_VIOLATION", "Periode ini harus disetujui SDM lain, bukan yang menghitung atau mengajukannya.")
		}
		now := pgconv.Time(s.now())
		if err := q.SetPayrollPeriodFinalized(ctx, store.SetPayrollPeriodFinalizedParams{ID: id, FinalizedBy: entry.UserID, FinalizedAt: now}); err != nil {
			return err
		}
		if err := q.PublishPayrolls(ctx, store.PublishPayrollsParams{PeriodID: id, PublishedAt: now}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollFinalized, "payroll_period", id
		return audit.Write(ctx, q, entry)
	})
}

// MarkPaid mencatat ubar periode final sudah ditransfer.
func (s *Service) MarkPaid(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, id)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusFinal {
			return statusError("Hanya periode final yang bisa ditandai dibayar.")
		}
		if err := q.SetPayrollPeriodPaid(ctx, store.SetPayrollPeriodPaidParams{ID: id, PaidBy: entry.UserID, PaidAt: pgconv.Time(s.now())}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollPaid, "payroll_period", id
		return audit.Write(ctx, q, entry)
	})
}

// PayrollRow adalah ringkasan slip satu Pejuang di detail periode.
type PayrollRow struct {
	EmployeeID  string            `json:"employee_id"`
	Nip         string            `json:"nip"`
	FullName    string            `json:"full_name"`
	Division    string            `json:"division"`
	Subdivision string            `json:"subdivision"`
	BaseSalary  string            `json:"base_salary"`
	Gross       string            `json:"gross"`
	Deductions  string            `json:"deductions"`
	Net         string            `json:"net"`
	NetClamped  bool              `json:"net_clamped"`
	Attendance  AttendanceSummary `json:"attendance"`
	HasInputs   bool              `json:"has_inputs"`
}

// ItemView adalah satu baris rincian slip.
type ItemView struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Quantity   *string `json:"quantity"`
	Rate       *string `json:"rate"`
	Amount     string  `json:"amount"`
	ShowOnSlip bool    `json:"show_on_slip"`
	Manual     bool    `json:"manual"`
}

// PeriodDetail adalah respons GET periode.
type PeriodDetail struct {
	Period     PeriodView   `json:"period"`
	Rows       []PayrollRow `json:"rows"`
	Totals     Totals       `json:"totals"`
	Readiness  *Readiness   `json:"readiness"`
	Components []Component  `json:"components"`
	// Items berisi total per kode komponen untuk ekspor dan ringkasan.
	ItemTotals []ItemTotal `json:"item_totals"`
}

// Totals menjumlahkan seluruh slip periode.
type Totals struct {
	BaseSalary string `json:"base_salary"`
	Gross      string `json:"gross"`
	Deductions string `json:"deductions"`
	Net        string `json:"net"`
}

// ItemTotal adalah jumlah satu komponen di seluruh slip, beserta nominal per Pejuang.
type ItemTotal struct {
	Code       string            `json:"code"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Total      string            `json:"total"`
	ByEmployee map[string]string `json:"by_employee"`
}

func decodeSnapshot(raw []byte) (Snapshot, error) {
	var snap Snapshot
	if len(raw) == 0 {
		return snap, nil
	}
	err := json.Unmarshal(raw, &snap)
	return snap, err
}

func (s *Service) PeriodDetail(ctx context.Context, viewer uuid.UUID, id uuid.UUID) (PeriodDetail, error) {
	p, err := s.q.GetPayrollPeriod(ctx, id)
	if pgconv.IsNoRows(err) {
		return PeriodDetail{}, httpx.ErrNotFound()
	}
	if err != nil {
		return PeriodDetail{}, err
	}
	rows, err := s.q.ListPayrollsOfPeriod(ctx, id)
	if err != nil {
		return PeriodDetail{}, err
	}
	inputRows, err := s.q.ListPayrollInputsOfPeriod(ctx, id)
	if err != nil {
		return PeriodDetail{}, err
	}
	inputs := inputsByEmployee(inputRows)
	items, err := s.q.ListPayrollItemsOfPeriod(ctx, id)
	if err != nil {
		return PeriodDetail{}, err
	}
	snap, err := decodeSnapshot(p.RulesSnapshot)
	if err != nil {
		return PeriodDetail{}, err
	}
	employeeOf := map[uuid.UUID]string{}
	out := PeriodDetail{Rows: make([]PayrollRow, 0, len(rows)), Components: snap.Components}
	if out.Components == nil {
		out.Components = []Component{}
	}
	var base, gross, deductions, net decimal.Decimal
	for _, r := range rows {
		var att AttendanceSummary
		if err := json.Unmarshal(r.Attendance, &att); err != nil {
			return PeriodDetail{}, err
		}
		employeeOf[r.ID] = r.EmployeeID.String()
		out.Rows = append(out.Rows, PayrollRow{
			EmployeeID: r.EmployeeID.String(), Nip: r.Nip, FullName: r.FullName, Division: r.Division, Subdivision: r.Subdivision,
			BaseSalary: r.BaseSalary.StringFixed(2), Gross: r.Gross.StringFixed(2), Deductions: r.Deductions.StringFixed(2),
			Net: r.Net.StringFixed(2), NetClamped: r.NetClamped, Attendance: att, HasInputs: len(inputs[r.EmployeeID]) > 0,
		})
		base, gross, deductions, net = base.Add(r.BaseSalary), gross.Add(r.Gross), deductions.Add(r.Deductions), net.Add(r.Net)
	}
	out.Totals = Totals{BaseSalary: base.StringFixed(2), Gross: gross.StringFixed(2), Deductions: deductions.StringFixed(2), Net: net.StringFixed(2)}
	byCode := map[string]*ItemTotal{}
	order := []string{}
	sums := map[string]decimal.Decimal{}
	for _, it := range items {
		t, ok := byCode[it.ComponentCode]
		if !ok {
			t = &ItemTotal{Code: it.ComponentCode, Name: it.Name, Kind: string(it.Kind), ByEmployee: map[string]string{}}
			byCode[it.ComponentCode] = t
			order = append(order, it.ComponentCode)
		}
		t.ByEmployee[employeeOf[it.PayrollID]] = it.Amount.StringFixed(2)
		sums[it.ComponentCode] = sums[it.ComponentCode].Add(it.Amount)
	}
	out.ItemTotals = make([]ItemTotal, 0, len(order))
	for _, code := range order {
		t := byCode[code]
		t.Total = sums[code].StringFixed(2)
		out.ItemTotals = append(out.ItemTotals, *t)
	}
	out.Period = periodView(p, viewer, int64(len(rows)), net)
	if string(p.Status) == StatusDraft {
		r, err := s.readiness(ctx, s.q, p)
		if err != nil {
			return PeriodDetail{}, err
		}
		out.Readiness = &r
	}
	return out, nil
}

// InputView adalah isian SDM untuk satu komponen per_unit atau manual.
type InputView struct {
	ComponentID string  `json:"component_id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Method      string  `json:"method"`
	Rate        *string `json:"rate"`
	Quantity    *string `json:"quantity"`
	Amount      *string `json:"amount"`
	Note        string  `json:"note"`
}

// EmployeePayroll adalah rincian slip satu Pejuang dalam periode.
type EmployeePayroll struct {
	Period PeriodView  `json:"period"`
	Row    PayrollRow  `json:"row"`
	Items  []ItemView  `json:"items"`
	Inputs []InputView `json:"inputs"`
}

func itemViews(items []store.PayrollItem) []ItemView {
	out := make([]ItemView, 0, len(items))
	for _, it := range items {
		out = append(out, ItemView{
			Code: it.ComponentCode, Name: it.Name, Kind: string(it.Kind), Quantity: nullString(it.Quantity, 2), Rate: nullString(it.Rate, 2),
			Amount: it.Amount.StringFixed(2), ShowOnSlip: it.ShowOnSlip, Manual: it.Manual,
		})
	}
	return out
}

func (s *Service) EmployeePayroll(ctx context.Context, viewer, periodID, employeeID uuid.UUID) (EmployeePayroll, error) {
	p, err := s.q.GetPayrollPeriod(ctx, periodID)
	if pgconv.IsNoRows(err) {
		return EmployeePayroll{}, httpx.ErrNotFound()
	}
	if err != nil {
		return EmployeePayroll{}, err
	}
	r, err := s.q.GetPayrollOfEmployee(ctx, store.GetPayrollOfEmployeeParams{PeriodID: periodID, EmployeeID: employeeID})
	if pgconv.IsNoRows(err) {
		return EmployeePayroll{}, httpx.ErrNotFound()
	}
	if err != nil {
		return EmployeePayroll{}, err
	}
	items, err := s.q.ListPayrollItems(ctx, r.ID)
	if err != nil {
		return EmployeePayroll{}, err
	}
	inputRows, err := s.q.ListPayrollInputsOfEmployee(ctx, store.ListPayrollInputsOfEmployeeParams{PeriodID: periodID, EmployeeID: employeeID})
	if err != nil {
		return EmployeePayroll{}, err
	}
	snap, err := decodeSnapshot(p.RulesSnapshot)
	if err != nil {
		return EmployeePayroll{}, err
	}
	var att AttendanceSummary
	if err := json.Unmarshal(r.Attendance, &att); err != nil {
		return EmployeePayroll{}, err
	}
	saved := map[uuid.UUID]store.PayrollInput{}
	for _, in := range inputRows {
		saved[in.ComponentID] = in
	}
	out := EmployeePayroll{
		Period: periodView(p, viewer, 0, decimal.Zero),
		Row: PayrollRow{
			EmployeeID: r.EmployeeID.String(), Nip: r.Nip, FullName: r.FullName, Division: r.Division, Subdivision: r.Subdivision,
			BaseSalary: r.BaseSalary.StringFixed(2), Gross: r.Gross.StringFixed(2), Deductions: r.Deductions.StringFixed(2),
			Net: r.Net.StringFixed(2), NetClamped: r.NetClamped, Attendance: att, HasInputs: len(inputRows) > 0,
		},
		Items:  itemViews(items),
		Inputs: []InputView{},
	}
	for _, c := range snap.Components {
		if c.Method != MethodPerUnit && c.Method != MethodManual {
			continue
		}
		v := InputView{ComponentID: c.ID, Code: c.Code, Name: c.Name, Kind: c.Kind, Method: c.Method, Rate: nullString(c.Rate, 2)}
		if id, err := uuid.Parse(c.ID); err == nil {
			if in, ok := saved[id]; ok {
				v.Quantity, v.Amount, v.Note = nullString(in.Quantity, 2), nullString(in.Amount, 2), in.Note
			}
		}
		out.Inputs = append(out.Inputs, v)
	}
	return out, nil
}

// InputItem adalah satu isian; kosongkan quantity dan amount untuk menghapusnya.
type InputItem struct {
	ComponentID string  `json:"component_id"`
	Quantity    *string `json:"quantity"`
	Amount      *string `json:"amount"`
	Note        string  `json:"note"`
}

// InputsInput mengganti seluruh isian satu Pejuang di periode.
type InputsInput struct {
	Inputs []InputItem `json:"inputs"`
}

func parseDecimal(raw *string) (decimal.NullDecimal, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return decimal.NullDecimal{}, true
	}
	v, err := decimal.NewFromString(strings.TrimSpace(*raw))
	if err != nil || v.IsNegative() {
		return decimal.NullDecimal{}, false
	}
	return decimal.NewNullDecimal(v.Round(2)), true
}

// SetInputs menyimpan isian lembur atau manual lalu menghitung ulang slip Pejuang itu saja.
func (s *Service) SetInputs(ctx context.Context, entry audit.Entry, periodID, employeeID uuid.UUID, in InputsInput) (EmployeePayroll, error) {
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		p, err := lockPeriod(ctx, q, periodID)
		if err != nil {
			return err
		}
		if string(p.Status) != StatusDraft {
			return statusError("Isian hanya bisa diubah saat periode masih draft.")
		}
		r, err := q.GetPayrollOfEmployee(ctx, store.GetPayrollOfEmployeeParams{PeriodID: periodID, EmployeeID: employeeID})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		snap, err := decodeSnapshot(p.RulesSnapshot)
		if err != nil {
			return err
		}
		byID := map[string]Component{}
		for _, c := range snap.Components {
			if c.Method == MethodPerUnit || c.Method == MethodManual {
				byID[c.ID] = c
			}
		}
		fields := map[string]string{}
		params := []store.InsertPayrollInputParams{}
		inputs := map[string]Input{}
		seen := map[string]bool{}
		for _, item := range in.Inputs {
			c, ok := byID[item.ComponentID]
			if !ok || seen[item.ComponentID] {
				fields["inputs"] = "Komponen tidak dikenal, bukan isian per periode, atau tercantum dua kali."
				break
			}
			seen[item.ComponentID] = true
			note := cleanText(item.Note)
			if utf8.RuneCountInString(note) > maxInputNote {
				fields[c.Code] = "Catatan maksimal 300 karakter."
				continue
			}
			var qty, amount decimal.NullDecimal
			if c.Method == MethodPerUnit {
				v, ok := parseDecimal(item.Quantity)
				if !ok || (v.Valid && v.Decimal.GreaterThan(decimal.NewFromInt(maxQuantity))) {
					fields[c.Code] = "Jumlah diisi angka 0 sampai 10.000."
					continue
				}
				qty = v
			} else {
				v, ok := parseDecimal(item.Amount)
				if !ok || (v.Valid && v.Decimal.GreaterThan(decimal.NewFromInt(maxAmount))) {
					fields[c.Code] = "Nominal diisi angka 0 sampai 1.000.000.000."
					continue
				}
				amount = v
			}
			if !qty.Valid && !amount.Valid {
				continue
			}
			id, _ := uuid.Parse(c.ID)
			params = append(params, store.InsertPayrollInputParams{
				PeriodID: periodID, EmployeeID: employeeID, ComponentID: id, Quantity: qty, Amount: amount, Note: note, UpdatedBy: entry.UserID,
			})
			inputs[c.ID] = Input{Quantity: qty, Amount: amount}
		}
		if len(fields) > 0 {
			return httpx.Validation(fields)
		}
		if err := q.DeletePayrollInputsOfEmployee(ctx, store.DeletePayrollInputsOfEmployeeParams{PeriodID: periodID, EmployeeID: employeeID}); err != nil {
			return err
		}
		for _, prm := range params {
			if err := q.InsertPayrollInput(ctx, prm); err != nil {
				return err
			}
		}
		var att AttendanceSummary
		if err := json.Unmarshal(r.Attendance, &att); err != nil {
			return err
		}
		values := map[string]decimal.Decimal{}
		if err := json.Unmarshal(r.EmployeeValues, &values); err != nil {
			return err
		}
		if err := q.DeletePayroll(ctx, store.DeletePayrollParams{PeriodID: periodID, EmployeeID: employeeID}); err != nil {
			return err
		}
		info := store.PayrollEmployeeInfoRow{
			ID: employeeID, Nip: r.Nip, FullName: r.FullName, Position: r.Position, BaseSalary: r.BaseSalary, Division: r.Division, Subdivision: r.Subdivision,
		}
		if err := writePayroll(ctx, q, periodID, snap, payrollSource{info: info, attendance: att, values: values, inputs: inputs}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.PayrollAdjusted, "payroll_period", periodID
		entry.Detail = map[string]any{"employee_id": employeeID.String(), "inputs": len(params)}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return EmployeePayroll{}, err
	}
	return s.EmployeePayroll(ctx, entry.UserID.UUID, periodID, employeeID)
}

// DateLocked melaporkan apakah ada tanggal dalam [first, last] yang masuk periode yang sudah diajukan, final,
// atau dibayar. Dipakai layanan presensi dan cuti sebelum mengubah presensi.
func DateLocked(ctx context.Context, q *store.Queries, first, last time.Time) (bool, error) {
	return q.AttendanceDateLocked(ctx, store.AttendanceDateLockedParams{FirstDate: pgconv.Date(first), LastDate: pgconv.Date(last)})
}

// ErrPeriodLocked adalah error presensi yang masuk periode terkunci.
func ErrPeriodLocked() error {
	return httpx.NewError(http.StatusConflict, "PERIOD_LOCKED", "Tanggal ini masuk periode ubar yang sudah diajukan atau final, jadi presensinya terkunci.")
}

// principalUser mengambil user pemanggil untuk menentukan can_finalize.
func principalUser(r *http.Request) uuid.UUID {
	p, _ := auth.PrincipalFrom(r.Context())
	return p.UserID
}
