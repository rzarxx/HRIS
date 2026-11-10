package leave

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/approval"
	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const dateLayout = "2006-01-02"

// ScopeResolver dihitung dari struktur organisasi (auth.Service.ResolveScope).
type ScopeResolver func(ctx context.Context, p auth.Principal, key string) (auth.ScopeFilter, error)

// Balance adalah saldo satu jenis cuti pada satu tahun kalender.
type Balance struct {
	Year          int     `json:"year"`
	EntitledDays  int     `json:"entitled_days"`
	UsedDays      int     `json:"used_days"`
	RemainingDays int     `json:"remaining_days"`
	EligibleFrom  *string `json:"eligible_from"`
}

// LeaveType adalah jenis cuti beserta saldo pemohon bila jenis itu bersaldo.
type LeaveType struct {
	ID              string   `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Paid            bool     `json:"paid"`
	QuotaKind       string   `json:"quota_kind"`
	AnnualQuotaDays *int     `json:"annual_quota_days"`
	IsActive        bool     `json:"is_active"`
	Balance         *Balance `json:"balance,omitempty"`
}

// TypeRef adalah jenis cuti di dalam pengajuan.
type TypeRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Request adalah pengajuan cuti/izin dengan timeline persetujuan (MASTER.md §7.6).
type Request struct {
	ID              string          `json:"id"`
	EmployeeID      string          `json:"employee_id"`
	EmployeeName    *string         `json:"employee_name,omitempty"`
	SubdivisionName *string         `json:"subdivision_name,omitempty"`
	LeaveType       TypeRef         `json:"leave_type"`
	StartDate       string          `json:"start_date"`
	EndDate         string          `json:"end_date"`
	Days            int             `json:"days"`
	Reason          string          `json:"reason"`
	Status          string          `json:"status"`
	CreatedAt       string          `json:"created_at"`
	DecidedAt       *string         `json:"decided_at"`
	Approvals       []approval.Step `json:"approvals"`
}

// CreateInput adalah isi pengajuan baru. Lampiran menyusul bersama modul unggah.
type CreateInput struct {
	LeaveTypeID string `json:"leave_type_id"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Reason      string `json:"reason"`
}

// TypeInput adalah isi jenis cuti dari SDM.
type TypeInput struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Paid            *bool  `json:"paid"`
	QuotaKind       string `json:"quota_kind"`
	AnnualQuotaDays *int   `json:"annual_quota_days"`
	IsActive        *bool  `json:"is_active"`
}

type Service struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	scope ScopeResolver
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool, scope ScopeResolver) *Service {
	return &Service{pool: pool, q: store.New(pool), scope: scope, now: time.Now}
}

// SetClock dipakai tes.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

func (s *Service) today() time.Time { return pgconv.WorkDate(s.now()) }

func presentType(t store.LeaveType) LeaveType {
	out := LeaveType{ID: t.ID.String(), Code: t.Code, Name: t.Name, Paid: t.Paid, QuotaKind: string(t.QuotaKind), IsActive: t.IsActive}
	if t.AnnualQuotaDays.Valid {
		v := int(t.AnnualQuotaDays.Int16)
		out.AnnualQuotaDays = &v
	}
	return out
}

func dateOf(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func yearRange(year int) (time.Time, time.Time) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(1, 0, 0)
}

func (s *Service) usedDays(ctx context.Context, q *store.Queries, employeeID uuid.UUID, year int) (map[uuid.UUID]int, error) {
	from, to := yearRange(year)
	rows, err := q.UsedLeaveDays(ctx, store.UsedLeaveDaysParams{EmployeeID: employeeID, YearStart: pgconv.Date(from), YearEnd: pgconv.Date(to)})
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.LeaveTypeID] = int(r.Days)
	}
	return out, nil
}

func balance(t store.LeaveType, join *time.Time, ref time.Time, used int) *Balance {
	entitled, from := Entitlement(string(t.QuotaKind), int(t.AnnualQuotaDays.Int16), join, ref)
	if entitled == Unlimited {
		return nil
	}
	b := &Balance{Year: ref.Year(), EntitledDays: entitled, UsedDays: used, RemainingDays: max(entitled-used, 0)}
	if from != nil {
		v := from.Format(dateLayout)
		b.EligibleFrom = &v
	}
	return b
}

// Types mengembalikan jenis cuti aktif beserta saldo tahun ini milik pemohon.
func (s *Service) Types(ctx context.Context, p auth.Principal) ([]LeaveType, error) {
	types, err := s.q.ListLeaveTypes(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := s.q.GetLeaveProfile(ctx, p.EmployeeID)
	if err != nil {
		return nil, err
	}
	today := s.today()
	used, err := s.usedDays(ctx, s.q, p.EmployeeID, today.Year())
	if err != nil {
		return nil, err
	}
	out := make([]LeaveType, 0, len(types))
	for _, t := range types {
		if !t.IsActive {
			continue
		}
		item := presentType(t)
		item.Balance = balance(t, dateOf(profile.JoinDate), today, used[t.ID])
		out = append(out, item)
	}
	return out, nil
}

type parsedCreate struct {
	typeID     uuid.UUID
	start, end time.Time
	reason     string
}

func (s *Service) parse(in CreateInput) (parsedCreate, error) {
	fields := map[string]string{}
	var p parsedCreate
	id, err := uuid.Parse(in.LeaveTypeID)
	if err != nil {
		fields["leave_type_id"] = "Pilih jenis cuti."
	}
	p.typeID = id
	start, err := time.Parse(dateLayout, in.StartDate)
	if err != nil {
		fields["start_date"] = "Tanggal mulai tidak valid."
	}
	end, err2 := time.Parse(dateLayout, in.EndDate)
	if err2 != nil {
		fields["end_date"] = "Tanggal selesai tidak valid."
	}
	p.start, p.end = start, end
	if err == nil && err2 == nil {
		today := s.today()
		switch {
		case end.Before(start):
			fields["end_date"] = "Tanggal selesai tidak boleh sebelum tanggal mulai."
		case start.Year() != end.Year():
			fields["end_date"] = "Pisahkan pengajuan yang melewati pergantian tahun."
		case end.Sub(start) >= maxRangeDays*24*time.Hour:
			fields["end_date"] = fmt.Sprintf("Satu pengajuan paling lama %d hari.", maxRangeDays)
		case start.Before(today.AddDate(0, 0, -maxBackDays)):
			fields["start_date"] = fmt.Sprintf("Tanggal mulai paling lambat %d hari ke belakang.", maxBackDays)
		}
	}
	p.reason = strings.TrimSpace(in.Reason)
	switch n := len([]rune(p.reason)); {
	case n < 3:
		fields["reason"] = "Alasan wajib diisi, minimal 3 karakter."
	case n > 500:
		fields["reason"] = "Alasan maksimal 500 karakter."
	}
	if len(fields) > 0 {
		return p, httpx.Validation(fields)
	}
	return p, nil
}

func (s *Service) holidays(ctx context.Context, q *store.Queries, start, end time.Time) (map[string]bool, error) {
	rows, err := q.HolidaysInRange(ctx, store.HolidaysInRangeParams{StartDate: pgconv.Date(start), EndDate: pgconv.Date(end)})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, d := range rows {
		out[d.Time.Format(dateLayout)] = true
	}
	return out, nil
}

func duplicateRequest(message string) error {
	return &httpx.Error{
		Status: http.StatusConflict, Code: "DUPLICATE_REQUEST", Message: message,
		Details: map[string]any{"fields": map[string]string{"start_date": message}},
	}
}

// Create menyimpan pengajuan dan membuat snapshot langkah persetujuan dalam satu transaksi.
func (s *Service) Create(ctx context.Context, p auth.Principal, ip *netip.Addr, in CreateInput) (Request, error) {
	parsed, err := s.parse(in)
	if err != nil {
		return Request{}, err
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.LockEmployeeForDevice(ctx, p.EmployeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		t, err := q.GetLeaveType(ctx, parsed.typeID)
		if pgconv.IsNoRows(err) || (err == nil && !t.IsActive) {
			return httpx.Validation(map[string]string{"leave_type_id": "Jenis cuti tidak tersedia."})
		}
		if err != nil {
			return err
		}
		profile, err := q.GetLeaveProfile(ctx, p.EmployeeID)
		if err != nil {
			return err
		}
		if len(profile.WorkDays) == 0 {
			return httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_ASSIGNED", "Shift Anda belum ditentukan. Hubungi SDM.")
		}
		overlap, err := q.CountOverlappingLeaves(ctx, store.CountOverlappingLeavesParams{
			EmployeeID: p.EmployeeID, StartDate: pgconv.Date(parsed.start), EndDate: pgconv.Date(parsed.end),
		})
		if err != nil {
			return err
		}
		if overlap > 0 {
			return duplicateRequest("Tanggal ini bertabrakan dengan pengajuan cuti lain.")
		}
		attended, err := q.AttendedDatesInRange(ctx, store.AttendedDatesInRangeParams{
			EmployeeID: p.EmployeeID, StartDate: pgconv.Date(parsed.start), EndDate: pgconv.Date(parsed.end),
		})
		if err != nil {
			return err
		}
		if len(attended) > 0 {
			return httpx.Validation(map[string]string{"start_date": "Sudah ada presensi masuk pada " + attended[0].Time.Format(dateLayout) + "."})
		}
		holidays, err := s.holidays(ctx, q, parsed.start, parsed.end)
		if err != nil {
			return err
		}
		days := len(WorkDays(parsed.start, parsed.end, profile.WorkDays, holidays))
		if days == 0 {
			return httpx.Validation(map[string]string{"end_date": "Rentang tanggal ini tidak berisi hari kerja."})
		}
		used, err := s.usedDays(ctx, q, p.EmployeeID, parsed.start.Year())
		if err != nil {
			return err
		}
		if b := balance(t, dateOf(profile.JoinDate), parsed.start, used[t.ID]); b != nil && days > b.RemainingDays {
			msg := fmt.Sprintf("Sisa %s tahun %d tinggal %d hari, pengajuan ini %d hari kerja.", t.Name, b.Year, b.RemainingDays, days)
			if t.QuotaKind == store.LeaveQuotaKindStatutory && b.EntitledDays == 0 {
				msg = "Hak cuti tahunan berlaku setelah 12 bulan bekerja."
				if b.EligibleFrom == nil {
					msg = "Tanggal masuk kerja belum diisi SDM, jadi hak cuti tahunan belum bisa dihitung."
				}
			}
			return httpx.Validation(map[string]string{"leave_type_id": msg})
		}
		row, err := q.InsertLeaveRequest(ctx, store.InsertLeaveRequestParams{
			EmployeeID: p.EmployeeID, LeaveTypeID: t.ID, StartDate: pgconv.Date(parsed.start), EndDate: pgconv.Date(parsed.end),
			Days: int16(days), Reason: parsed.reason,
		})
		if err != nil {
			return err
		}
		id = row.ID
		if err := approval.Start(ctx, q, store.RequestKindLeave, row.ID, p.EmployeeID); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.LeaveRequested, Entity: "leave", EntityID: row.ID, IP: ip,
			Detail: map[string]any{"leave_type": t.Code, "start_date": in.StartDate, "end_date": in.EndDate, "days": days},
		})
	})
	if err != nil {
		return Request{}, err
	}
	return s.detail(ctx, id)
}

func (s *Service) detail(ctx context.Context, id uuid.UUID) (Request, error) {
	r, err := s.q.GetLeaveDetail(ctx, id)
	if pgconv.IsNoRows(err) {
		return Request{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Request{}, err
	}
	out := Request{
		ID: r.ID.String(), EmployeeID: r.EmployeeID.String(), EmployeeName: &r.EmployeeName, SubdivisionName: &r.SubdivisionName,
		LeaveType: TypeRef{ID: r.LeaveTypeID.String(), Code: r.LeaveTypeCode, Name: r.LeaveTypeName},
		StartDate: r.StartDate.Time.Format(dateLayout), EndDate: r.EndDate.Time.Format(dateLayout), Days: int(r.Days),
		Reason: r.Reason, Status: string(r.Status), CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339), DecidedAt: pgconv.TimeString(r.DecidedAt),
	}
	if out.Approvals, err = approval.Timeline(ctx, s.q, store.RequestKindLeave, r.ID); err != nil {
		return Request{}, err
	}
	return out, nil
}

// Mine mengembalikan pengajuan milik pemohon, terbaru dulu, dengan timeline.
func (s *Service) Mine(ctx context.Context, p auth.Principal) ([]Request, error) {
	rows, err := s.q.ListMyLeaves(ctx, p.EmployeeID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	timelines, err := approval.Timelines(ctx, s.q, store.RequestKindLeave, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Request, 0, len(rows))
	for _, r := range rows {
		out = append(out, Request{
			ID: r.ID.String(), EmployeeID: r.EmployeeID.String(),
			LeaveType: TypeRef{ID: r.LeaveTypeID.String(), Code: r.LeaveTypeCode, Name: r.LeaveTypeName},
			StartDate: r.StartDate.Time.Format(dateLayout), EndDate: r.EndDate.Time.Format(dateLayout), Days: int(r.Days),
			Reason: r.Reason, Status: string(r.Status), CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
			DecidedAt: pgconv.TimeString(r.DecidedAt), Approvals: stepsOrEmpty(timelines[r.ID]),
		})
	}
	return out, nil
}

func stepsOrEmpty(s []approval.Step) []approval.Step {
	if s == nil {
		return []approval.Step{}
	}
	return s
}

// Get mengembalikan satu pengajuan milik pemohon.
func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Request, error) {
	out, err := s.detail(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if out.EmployeeID != p.EmployeeID.String() {
		return Request{}, httpx.ErrNotFound()
	}
	return out, nil
}

// Cancel membatalkan pengajuan yang masih menunggu; langkah tersisa dilewati.
func (s *Service) Cancel(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID) (Request, error) {
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.LockLeaveRequest(ctx, id)
		if pgconv.IsNoRows(err) || (err == nil && row.EmployeeID != p.EmployeeID) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		if row.Status != store.RequestStatusPending {
			return httpx.NewError(http.StatusConflict, "REQUEST_NOT_PENDING", "Pengajuan ini sudah diputuskan atau dibatalkan.")
		}
		if err := q.SetLeaveStatus(ctx, store.SetLeaveStatusParams{ID: id, Status: store.RequestStatusCancelled, DecidedAt: pgconv.Time(s.now())}); err != nil {
			return err
		}
		if err := approval.Close(ctx, q, store.RequestKindLeave, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{UserID: audit.Actor(p.UserID), Action: audit.LeaveCancelled, Entity: "leave", EntityID: id, IP: ip})
	})
	if err != nil {
		return Request{}, err
	}
	return s.detail(ctx, id)
}

// Lock, Decided, dan Summaries memasang cuti ke mesin approval.
func (s *Service) Lock(ctx context.Context, q *store.Queries, id uuid.UUID) (store.RequestStatus, error) {
	row, err := q.LockLeaveRequest(ctx, id)
	if pgconv.IsNoRows(err) {
		return "", httpx.ErrNotFound()
	}
	return row.Status, err
}

// Decided menyelesaikan pengajuan. Bila disetujui, hari kerja dalam rentang tercatat sebagai
// cuti di presensi (F13); hari yang sudah ada presensi masuk dibiarkan.
func (s *Service) Decided(ctx context.Context, q *store.Queries, id uuid.UUID, status store.RequestStatus, at time.Time) error {
	if err := q.SetLeaveStatus(ctx, store.SetLeaveStatusParams{ID: id, Status: status, DecidedAt: pgconv.Time(at)}); err != nil {
		return err
	}
	if status != store.RequestStatusApproved {
		return nil
	}
	row, err := q.LockLeaveRequest(ctx, id)
	if err != nil {
		return err
	}
	profile, err := q.GetLeaveProfile(ctx, row.EmployeeID)
	if err != nil {
		return err
	}
	holidays, err := s.holidays(ctx, q, row.StartDate.Time, row.EndDate.Time)
	if err != nil {
		return err
	}
	for _, d := range WorkDays(row.StartDate.Time, row.EndDate.Time, profile.WorkDays, holidays) {
		if err := q.MarkLeaveAttendance(ctx, store.MarkLeaveAttendanceParams{EmployeeID: row.EmployeeID, WorkDate: pgconv.Date(d)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Summaries(ctx context.Context, q *store.Queries, ids []uuid.UUID) (map[uuid.UUID]map[string]any, error) {
	rows, err := q.LeaveSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]map[string]any, len(rows))
	for _, r := range rows {
		out[r.ID] = map[string]any{
			"leave_type": r.LeaveTypeName, "start_date": r.StartDate.Time.Format(dateLayout),
			"end_date": r.EndDate.Time.Format(dateLayout), "days": int(r.Days), "reason": r.Reason,
		}
	}
	return out, nil
}

// AdminList mengembalikan pengajuan dalam scope pemanggil (leave:read).
func (s *Service) AdminList(ctx context.Context, p auth.Principal, status, from, to string) ([]Request, error) {
	f, err := s.scope(ctx, p, permission.LeaveRead)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	var st store.NullRequestStatus
	if status != "" {
		st = store.NullRequestStatus{RequestStatus: store.RequestStatus(status), Valid: true}
		switch st.RequestStatus {
		case store.RequestStatusPending, store.RequestStatusApproved, store.RequestStatusRejected, store.RequestStatusCancelled:
		default:
			fields["status"] = "Status tidak dikenal."
		}
	}
	year := s.today().Year()
	fromDate, toDate := pgconv.Date(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)), pgconv.Date(time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC))
	if from != "" {
		if fromDate, err = pgconv.ParseDate(from); err != nil {
			fields["from"] = "Tanggal tidak valid."
		}
	}
	if to != "" {
		if toDate, err = pgconv.ParseDate(to); err != nil {
			fields["to"] = "Tanggal tidak valid."
		}
	}
	if len(fields) > 0 {
		return nil, httpx.Validation(fields)
	}
	subs, divs := f.SubdivisionIDs, f.DivisionIDs
	if subs == nil {
		subs = []uuid.UUID{}
	}
	if divs == nil {
		divs = []uuid.UUID{}
	}
	rows, err := s.q.ListLeavesAdmin(ctx, store.ListLeavesAdminParams{
		ScopeAll: f.All, ScopeSubdivisionIds: subs, ScopeDivisionIds: divs, Status: st, FromDate: fromDate, ToDate: toDate,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	timelines, err := approval.Timelines(ctx, s.q, store.RequestKindLeave, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Request, 0, len(rows))
	for _, r := range rows {
		name, sub := r.EmployeeName, r.SubdivisionName
		out = append(out, Request{
			ID: r.ID.String(), EmployeeID: r.EmployeeID.String(), EmployeeName: &name, SubdivisionName: &sub,
			LeaveType: TypeRef{ID: r.LeaveTypeID.String(), Code: r.LeaveTypeCode, Name: r.LeaveTypeName},
			StartDate: r.StartDate.Time.Format(dateLayout), EndDate: r.EndDate.Time.Format(dateLayout), Days: int(r.Days),
			Reason: r.Reason, Status: string(r.Status), CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
			DecidedAt: pgconv.TimeString(r.DecidedAt), Approvals: stepsOrEmpty(timelines[r.ID]),
		})
	}
	return out, nil
}

// AdminTypes mengembalikan semua jenis cuti, termasuk yang nonaktif.
func (s *Service) AdminTypes(ctx context.Context) ([]LeaveType, error) {
	rows, err := s.q.ListLeaveTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LeaveType, 0, len(rows))
	for _, r := range rows {
		out = append(out, presentType(r))
	}
	return out, nil
}

type parsedType struct {
	quota  store.LeaveQuotaKind
	annual pgtype.Int2
}

func (in TypeInput) parse(requireCode bool) (parsedType, error) {
	fields := map[string]string{}
	var p parsedType
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 60 {
		fields["name"] = "Nama wajib diisi, maksimal 60 karakter."
	}
	if requireCode && !validCode(in.Code) {
		fields["code"] = "Kode berupa huruf kecil, angka, atau garis bawah, 2 sampai 32 karakter."
	}
	p.quota = store.LeaveQuotaKind(in.QuotaKind)
	switch p.quota {
	case store.LeaveQuotaKindCustom:
		if in.AnnualQuotaDays == nil || *in.AnnualQuotaDays < 1 || *in.AnnualQuotaDays > 366 {
			fields["annual_quota_days"] = "Jatah per tahun 1 sampai 366 hari."
		} else {
			p.annual = pgtype.Int2{Int16: int16(*in.AnnualQuotaDays), Valid: true}
		}
	case store.LeaveQuotaKindStatutory, store.LeaveQuotaKindNone:
	default:
		fields["quota_kind"] = "Pilih statutory, custom, atau none."
	}
	if len(fields) > 0 {
		return p, httpx.Validation(fields)
	}
	return p, nil
}

func validCode(code string) bool {
	if len(code) < 2 || len(code) > 32 || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	for _, c := range code {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func orTrue(b *bool) bool { return b == nil || *b }

// CreateType menambah jenis cuti.
func (s *Service) CreateType(ctx context.Context, p auth.Principal, ip *netip.Addr, in TypeInput) (LeaveType, error) {
	parsed, err := in.parse(true)
	if err != nil {
		return LeaveType{}, err
	}
	var row store.LeaveType
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err = q.InsertLeaveType(ctx, store.InsertLeaveTypeParams{
			Code: in.Code, Name: strings.TrimSpace(in.Name), Paid: orTrue(in.Paid), QuotaKind: parsed.quota,
			AnnualQuotaDays: parsed.annual, IsActive: orTrue(in.IsActive),
		})
		if err != nil {
			if _, ok := pgconv.UniqueConstraint(err); ok {
				return httpx.Duplicate("code", "Kode jenis cuti sudah dipakai.")
			}
			return err
		}
		return audit.Write(ctx, q, typeAudit(p, ip, row, "created"))
	})
	if err != nil {
		return LeaveType{}, err
	}
	return presentType(row), nil
}

// UpdateType mengubah jenis cuti. Kode tetap; pengajuan lama tetap memakai jumlah harinya.
func (s *Service) UpdateType(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in TypeInput) (LeaveType, error) {
	parsed, err := in.parse(false)
	if err != nil {
		return LeaveType{}, err
	}
	var row store.LeaveType
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err = q.UpdateLeaveType(ctx, store.UpdateLeaveTypeParams{
			ID: id, Name: strings.TrimSpace(in.Name), Paid: orTrue(in.Paid), QuotaKind: parsed.quota,
			AnnualQuotaDays: parsed.annual, IsActive: orTrue(in.IsActive),
		})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, typeAudit(p, ip, row, "updated"))
	})
	if err != nil {
		return LeaveType{}, err
	}
	return presentType(row), nil
}

func typeAudit(p auth.Principal, ip *netip.Addr, t store.LeaveType, change string) audit.Entry {
	return audit.Entry{
		UserID: audit.Actor(p.UserID), Action: audit.LeaveTypeChanged, Entity: "leave_type", EntityID: t.ID, IP: ip,
		Detail: map[string]any{"change": change, "code": t.Code, "quota_kind": t.QuotaKind, "annual_quota_days": t.AnnualQuotaDays.Int16, "is_active": t.IsActive},
	}
}
