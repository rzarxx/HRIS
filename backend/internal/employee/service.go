// Package employee mengelola data Pejuang.
package employee

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var (
	nipPattern    = regexp.MustCompile(`^[A-Za-z0-9-]{4,20}$`)
	phonePattern  = regexp.MustCompile(`^(\+62|0)8[0-9]{7,12}$`)
	emailPattern  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	moneyPattern  = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)
	maxBaseSalary = decimal.NewFromInt(1_000_000_000)
)

// ScopeResolver dihitung dari struktur organisasi (auth.Service.ResolveScope).
type ScopeResolver func(ctx context.Context, p auth.Principal, key string) (auth.ScopeFilter, error)

type Service struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	scope ScopeResolver
}

func NewService(pool *pgxpool.Pool, scope ScopeResolver) *Service {
	return &Service{pool: pool, q: store.New(pool), scope: scope}
}

// Employee adalah objek baku (MASTER.md §7.6). base_salary hanya untuk sdm dan pemilik data.
type Employee struct {
	ID               string   `json:"id"`
	Nip              string   `json:"nip"`
	FullName         string   `json:"full_name"`
	Email            *string  `json:"email"`
	Phone            *string  `json:"phone"`
	SubdivisionID    string   `json:"subdivision_id"`
	SubdivisionName  string   `json:"subdivision_name"`
	DivisionID       string   `json:"division_id"`
	DivisionName     string   `json:"division_name"`
	Position         *string  `json:"position"`
	JoinDate         *string  `json:"join_date"`
	Status           string   `json:"status"`
	DefaultShiftID   *string  `json:"default_shift_id"`
	DefaultShiftName *string  `json:"default_shift_name"`
	BaseSalary       *string  `json:"base_salary,omitempty"`
	Account          *Account `json:"account,omitempty"`
}

// Account adalah ringkasan akun, hanya untuk pemegang user:manage.
type Account struct {
	UserID     string `json:"user_id"`
	Role       string `json:"role"`
	IsActive   bool   `json:"is_active"`
	IsLocked   bool   `json:"is_locked"`
	MFAEnabled bool   `json:"mfa_enabled"`
}

// Created adalah respons pembuatan Pejuang, memuat undangan aktivasi sekali tampil.
type Created struct {
	Employee
	Activation auth.Activation `json:"activation"`
}

// Input juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya.
type Input struct {
	Nip            *string `json:"nip"`
	FullName       string  `json:"full_name"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	SubdivisionID  string  `json:"subdivision_id"`
	Position       *string `json:"position"`
	JoinDate       *string `json:"join_date"`
	BaseSalary     string  `json:"base_salary"`
	Status         string  `json:"status"`
	DefaultShiftID *string `json:"default_shift_id"`
}

type validated struct {
	Nip            string
	FullName       string
	Email          pgtype.Text
	Phone          pgtype.Text
	SubdivisionID  uuid.UUID
	Position       pgtype.Text
	JoinDate       pgtype.Date
	BaseSalary     decimal.Decimal
	Status         string
	DefaultShiftID uuid.NullUUID
}

func (in Input) validate() (validated, error) {
	fields := map[string]string{}
	v := validated{
		FullName: strings.TrimSpace(in.FullName),
		Email:    pgconv.Text(in.Email),
		Phone:    pgconv.Text(in.Phone),
		Position: pgconv.Text(in.Position),
		Status:   in.Status,
	}
	if in.Nip != nil {
		v.Nip = strings.ToUpper(strings.TrimSpace(*in.Nip))
		if v.Nip != "" && !nipPattern.MatchString(v.Nip) {
			fields["nip"] = "NIPY 4-20 karakter huruf, angka, atau tanda minus."
		}
	}
	switch n := len(v.FullName); {
	case n == 0:
		fields["full_name"] = "Nama lengkap wajib diisi."
	case n < 3:
		fields["full_name"] = "Nama minimal 3 karakter."
	case n > 100:
		fields["full_name"] = "Nama maksimal 100 karakter."
	}
	// Email menjadi identitas login sehingga wajib (F3).
	switch {
	case !v.Email.Valid:
		fields["email"] = "Email wajib diisi."
	case len(v.Email.String) > 254 || !emailPattern.MatchString(v.Email.String):
		fields["email"] = "Format email tidak valid."
	}
	if v.Phone.Valid && !phonePattern.MatchString(v.Phone.String) {
		fields["phone"] = "Nomor HP tidak valid (contoh 081234567890)."
	}
	if v.Position.Valid && len(v.Position.String) > 100 {
		fields["position"] = "Jabatan maksimal 100 karakter."
	}
	id, err := uuid.Parse(in.SubdivisionID)
	if err != nil {
		fields["subdivision_id"] = "Pilih subdiv."
	}
	v.SubdivisionID = id
	if in.JoinDate != nil && strings.TrimSpace(*in.JoinDate) != "" {
		d, err := pgconv.ParseDate(strings.TrimSpace(*in.JoinDate))
		if err != nil {
			fields["join_date"] = "Tanggal tidak valid."
		}
		v.JoinDate = d
	}
	switch {
	case in.BaseSalary == "":
		fields["base_salary"] = "Gaji pokok wajib diisi."
	case !moneyPattern.MatchString(in.BaseSalary):
		fields["base_salary"] = "Gaji pokok harus angka, contoh 4500000.00."
	default:
		amount, err := decimal.NewFromString(in.BaseSalary)
		if err != nil || amount.GreaterThan(maxBaseSalary) {
			fields["base_salary"] = "Gaji pokok terlalu besar."
		}
		v.BaseSalary = amount
	}
	if v.Status != "active" && v.Status != "inactive" {
		fields["status"] = "Status tidak valid."
	}
	shiftID, err := pgconv.ParseOptionalUUID(in.DefaultShiftID)
	if err != nil {
		fields["default_shift_id"] = "Pilih shift."
	}
	v.DefaultShiftID = shiftID
	if len(fields) > 0 {
		return v, httpx.Validation(fields)
	}
	return v, nil
}

func toFilter(f auth.ScopeFilter) (bool, []uuid.UUID, []uuid.UUID) {
	subs, divs := f.SubdivisionIDs, f.DivisionIDs
	if subs == nil {
		subs = []uuid.UUID{}
	}
	if divs == nil {
		divs = []uuid.UUID{}
	}
	return f.All, subs, divs
}

func mapUnique(err error) error {
	constraint, ok := pgconv.UniqueConstraint(err)
	if !ok {
		return err
	}
	if strings.Contains(constraint, "email") {
		return httpx.Duplicate("email", "Email sudah dipakai Pejuang lain.")
	}
	return httpx.Duplicate("nip", "NIPY sudah dipakai Pejuang lain.")
}

// ListParams adalah filter resmi daftar Pejuang.
type ListParams struct {
	Q             string
	DivisionID    uuid.NullUUID
	SubdivisionID uuid.NullUUID
	Status        string
	Page          httpx.Page
}

func (s *Service) attachAccounts(ctx context.Context, p auth.Principal, employees []Employee, ids []uuid.UUID) error {
	if !permission.Allows(p.Role, permission.UserManage) || len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListEmployeeAccounts(ctx, ids)
	if err != nil {
		return err
	}
	accounts := make(map[string]*Account, len(rows))
	for _, r := range rows {
		accounts[r.EmployeeID.String()] = &Account{
			UserID:     r.UserID.String(),
			Role:       string(r.Role),
			IsActive:   r.IsActive,
			IsLocked:   r.LockedUntil.Valid && r.LockedUntil.Time.After(time.Now()),
			MFAEnabled: r.MfaEnabled,
		}
	}
	for i := range employees {
		employees[i].Account = accounts[employees[i].ID]
	}
	return nil
}

func (s *Service) List(ctx context.Context, p auth.Principal, params ListParams) ([]Employee, int64, error) {
	scope, err := s.scope(ctx, p, permission.EmployeeRead)
	if err != nil {
		return nil, 0, err
	}
	all, subs, divs := toFilter(scope)
	q := pgtype.Text{}
	if params.Q != "" {
		q = pgtype.Text{String: params.Q, Valid: true}
	}
	status := pgtype.Text{}
	if params.Status != "" {
		status = pgtype.Text{String: params.Status, Valid: true}
	}
	total, err := s.q.CountEmployees(ctx, store.CountEmployeesParams{
		ScopeAll: all, ScopeSubdivisionIds: subs, ScopeDivisionIds: divs,
		Q: q, DivisionID: params.DivisionID, SubdivisionID: params.SubdivisionID, Status: status,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListEmployees(ctx, store.ListEmployeesParams{
		ScopeAll: all, ScopeSubdivisionIds: subs, ScopeDivisionIds: divs,
		Q: q, DivisionID: params.DivisionID, SubdivisionID: params.SubdivisionID, Status: status,
		PageLimit: params.Page.Limit(), PageOffset: params.Page.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Employee, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, present(p, store.GetEmployeeScopedRow(r)))
		ids = append(ids, r.ID)
	}
	if err := s.attachAccounts(ctx, p, out, ids); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Get menjawab 404 untuk objek di luar scope agar keberadaan data tidak bocor (§3.3).
func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Employee, error) {
	row, err := s.getScoped(ctx, p, id)
	if err != nil {
		return Employee{}, err
	}
	out := []Employee{present(p, row)}
	if err := s.attachAccounts(ctx, p, out, []uuid.UUID{id}); err != nil {
		return Employee{}, err
	}
	return out[0], nil
}

func (s *Service) getScoped(ctx context.Context, p auth.Principal, id uuid.UUID) (store.GetEmployeeScopedRow, error) {
	scope, err := s.scope(ctx, p, permission.EmployeeRead)
	if err != nil {
		return store.GetEmployeeScopedRow{}, err
	}
	all, subs, divs := toFilter(scope)
	row, err := s.q.GetEmployeeScoped(ctx, store.GetEmployeeScopedParams{
		ID: id, ScopeAll: all, ScopeSubdivisionIds: subs, ScopeDivisionIds: divs,
	})
	if pgconv.IsNoRows(err) {
		return row, httpx.ErrNotFound()
	}
	return row, err
}

func (s *Service) assertSubdivision(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.GetSubdivision(ctx, id)
	if pgconv.IsNoRows(err) {
		return httpx.Validation(map[string]string{"subdivision_id": "Subdiv tidak ditemukan."})
	}
	return err
}

func (s *Service) assertShift(ctx context.Context, id uuid.NullUUID) error {
	if !id.Valid {
		return nil
	}
	_, err := s.q.GetShift(ctx, id.UUID)
	if pgconv.IsNoRows(err) {
		return httpx.Validation(map[string]string{"default_shift_id": "Shift tidak ditemukan."})
	}
	return err
}

// Create membuat data Pejuang, akun belum aktif, dan undangan aktivasi (F3).
func (s *Service) Create(ctx context.Context, p auth.Principal, ip audit.Entry, in Input, now time.Time) (Created, error) {
	// Selain SDM tidak mengurus gaji: Pejuang dibuat dengan gaji 0 dan SDM yang mengisinya (D34).
	if !permission.ManagesSalary(p.Role) {
		if in.BaseSalary != "" {
			return Created{}, httpx.ErrForbidden()
		}
		in.BaseSalary = "0.00"
	}
	if in.Status == "" {
		in.Status = "active"
	}
	v, err := in.validate()
	if err != nil {
		return Created{}, err
	}
	if err := s.assertSubdivision(ctx, v.SubdivisionID); err != nil {
		return Created{}, err
	}
	if err := s.assertShift(ctx, v.DefaultShiftID); err != nil {
		return Created{}, err
	}
	// Argon2 sengaja lambat, jadi dihitung sebelum transaksi dibuka.
	unusable, err := auth.UnusablePasswordHash()
	if err != nil {
		return Created{}, err
	}
	var activation auth.Activation
	if !v.JoinDate.Valid {
		v.JoinDate = pgconv.Date(pgconv.WorkDate(now))
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		nip := v.Nip
		if nip == "" {
			seq, err := q.NextNipSequence(ctx)
			if err != nil {
				return err
			}
			nip = fmt.Sprintf("PJ%d%04d", now.In(pgconv.WIB).Year(), seq)
		}
		var err error
		id, err = q.CreateEmployee(ctx, store.CreateEmployeeParams{
			Nip: nip, FullName: v.FullName, Email: v.Email, Phone: v.Phone,
			SubdivisionID: v.SubdivisionID, Position: v.Position, JoinDate: v.JoinDate,
			BaseSalary: v.BaseSalary, Status: v.Status, DefaultShiftID: v.DefaultShiftID,
		})
		if err != nil {
			return mapUnique(err)
		}
		userID, err := q.CreateInactiveUser(ctx, store.CreateInactiveUserParams{
			EmployeeID: id, Email: v.Email.String, PasswordHash: unusable, Role: store.UserRolePejuang,
		})
		if err != nil {
			return mapUnique(err)
		}
		if activation, err = auth.IssueInvite(ctx, q, userID, now); err != nil {
			return err
		}
		entry := ip
		entry.Action, entry.Entity, entry.EntityID = audit.EmployeeCreated, "employee", id
		if err := audit.Write(ctx, q, entry); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.UserCreated, "user", userID
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return Created{}, err
	}
	employee, err := s.Get(ctx, p, id)
	if err != nil {
		return Created{}, err
	}
	return Created{Employee: employee, Activation: activation}, nil
}

// Update memuat data lama lebih dulu, lalu menimpa dengan field yang dikirim klien.
func (s *Service) Update(ctx context.Context, p auth.Principal, ip audit.Entry, id uuid.UUID, decode func(*Input) error) (Employee, error) {
	current, err := s.getScoped(ctx, p, id)
	if err != nil {
		return Employee{}, err
	}
	if err := auth.AssertEmployeeNotProtected(ctx, s.q, p, id); err != nil {
		return Employee{}, err
	}
	nip := current.Nip
	in := Input{
		Nip:            &nip,
		FullName:       current.FullName,
		Email:          pgconv.TextPtr(current.Email),
		Phone:          pgconv.TextPtr(current.Phone),
		SubdivisionID:  current.SubdivisionID.String(),
		Position:       pgconv.TextPtr(current.Position),
		JoinDate:       pgconv.DateString(current.JoinDate),
		BaseSalary:     current.BaseSalary.StringFixed(2),
		Status:         current.Status,
		DefaultShiftID: pgconv.UUIDPtr(current.DefaultShiftID),
	}
	if err := decode(&in); err != nil {
		return Employee{}, err
	}
	if !permission.ManagesSalary(p.Role) && in.BaseSalary != current.BaseSalary.StringFixed(2) {
		return Employee{}, httpx.ErrForbidden()
	}
	v, err := in.validate()
	if err != nil {
		return Employee{}, err
	}
	if v.Nip == "" {
		v.Nip = current.Nip
	}
	if err := s.assertSubdivision(ctx, v.SubdivisionID); err != nil {
		return Employee{}, err
	}
	if err := s.assertShift(ctx, v.DefaultShiftID); err != nil {
		return Employee{}, err
	}
	if v.Status == "inactive" && id == p.EmployeeID {
		return Employee{}, errSelfDeactivate()
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.UpdateEmployee(ctx, store.UpdateEmployeeParams{
			ID: id, Nip: v.Nip, FullName: v.FullName, Email: v.Email, Phone: v.Phone,
			SubdivisionID: v.SubdivisionID, Position: v.Position, JoinDate: v.JoinDate,
			BaseSalary: v.BaseSalary, Status: v.Status, DefaultShiftID: v.DefaultShiftID,
		})
		if err != nil {
			return mapUnique(err)
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		if v.Status == "inactive" || v.SubdivisionID != current.SubdivisionID {
			if err := q.ClearHeadOfEmployee(ctx, uuid.NullUUID{UUID: id, Valid: true}); err != nil {
				return err
			}
		}
		// Email login hanya ikut berubah bila email Pejuang memang diganti, karena keduanya bisa berbeda.
		if v.Email.String != current.Email.String {
			if err := q.UpdateUserEmail(ctx, store.UpdateUserEmailParams{EmployeeID: id, Email: v.Email.String}); err != nil {
				return mapUnique(err)
			}
		}
		if v.Status == "inactive" && current.Status != "inactive" {
			if err := offboardAccount(ctx, q, id); err != nil {
				return err
			}
		}
		entry := ip
		entry.Action, entry.Entity, entry.EntityID = audit.EmployeeUpdated, "employee", id
		if v.DefaultShiftID != current.DefaultShiftID {
			entry.Detail = map[string]any{"default_shift_id": pgconv.UUIDPtr(v.DefaultShiftID)}
		}
		if err := audit.Write(ctx, q, entry); err != nil {
			return err
		}
		if !v.BaseSalary.Equal(current.BaseSalary) {
			entry.Action = audit.EmployeeSalaryChanged
			entry.Detail = map[string]any{"from": current.BaseSalary.StringFixed(2), "to": v.BaseSalary.StringFixed(2)}
			return audit.Write(ctx, q, entry)
		}
		return nil
	})
	if err != nil {
		return Employee{}, err
	}
	return s.Get(ctx, p, id)
}

func errSelfDeactivate() error {
	return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Anda tidak dapat menonaktifkan akun sendiri.")
}

// Deactivate menonaktifkan Pejuang; riwayat presensi dan upah tetap tersimpan.
func (s *Service) Deactivate(ctx context.Context, p auth.Principal, ip audit.Entry, id uuid.UUID, now time.Time) (Employee, error) {
	if id == p.EmployeeID {
		return Employee{}, errSelfDeactivate()
	}
	if _, err := s.getScoped(ctx, p, id); err != nil {
		return Employee{}, err
	}
	if err := auth.AssertEmployeeNotProtected(ctx, s.q, p, id); err != nil {
		return Employee{}, err
	}
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.DeactivateEmployee(ctx, store.DeactivateEmployeeParams{ID: id, ExitDate: pgconv.Date(pgconv.WorkDate(now))})
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil // sudah nonaktif: idempoten
		}
		if err := q.ClearHeadOfEmployee(ctx, uuid.NullUUID{UUID: id, Valid: true}); err != nil {
			return err
		}
		if err := offboardAccount(ctx, q, id); err != nil {
			return err
		}
		ip.Action, ip.Entity, ip.EntityID = audit.EmployeeDeactivated, "employee", id
		return audit.Write(ctx, q, ip)
	})
	if err != nil {
		return Employee{}, err
	}
	return s.Get(ctx, p, id)
}

// offboardAccount menonaktifkan akun dan mencabut seluruh sesi (F20).
func offboardAccount(ctx context.Context, q *store.Queries, employeeID uuid.UUID) error {
	user, err := q.GetUserByEmployee(ctx, employeeID)
	if pgconv.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := q.SetUserActive(ctx, store.SetUserActiveParams{ID: user.ID, IsActive: false}); err != nil {
		return err
	}
	if err := q.InvalidateUserInvites(ctx, user.ID); err != nil {
		return err
	}
	if err := q.RevokeEmployeeDevices(ctx, employeeID); err != nil {
		return err
	}
	return q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{
		UserID: user.ID, Reason: pgtype.Text{String: "offboarded", Valid: true},
	})
}

func present(p auth.Principal, r store.GetEmployeeScopedRow) Employee {
	e := Employee{
		ID: r.ID.String(), Nip: r.Nip, FullName: r.FullName,
		Email: pgconv.TextPtr(r.Email), Phone: pgconv.TextPtr(r.Phone),
		SubdivisionID: r.SubdivisionID.String(), SubdivisionName: r.SubdivisionName,
		DivisionID: r.DivisionID.String(), DivisionName: r.DivisionName,
		Position: pgconv.TextPtr(r.Position), JoinDate: pgconv.DateString(r.JoinDate),
		Status:         r.Status,
		DefaultShiftID: pgconv.UUIDPtr(r.DefaultShiftID), DefaultShiftName: pgconv.TextPtr(r.DefaultShiftName),
	}
	if permission.ManagesSalary(p.Role) || r.ID == p.EmployeeID {
		salary := r.BaseSalary.StringFixed(2)
		e.BaseSalary = &salary
	}
	return e
}
