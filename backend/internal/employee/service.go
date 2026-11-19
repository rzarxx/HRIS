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
	"github.com/rzarxx/HRIS/backend/internal/platform/phone"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var (
	nipPattern    = regexp.MustCompile(`^[A-Za-z0-9.-]{4,20}$`)
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
	// photos menyimpan foto profil; nil bila SELFIE_DIR belum diisi.
	photos PhotoStore
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
	Gender           *string  `json:"gender"`
	MaritalStatus    *string  `json:"marital_status"`
	HasChildren      *bool    `json:"has_children"`
	ChildrenCount    *int     `json:"children_count"`
	SubdivisionID    string   `json:"subdivision_id"`
	SubdivisionName  string   `json:"subdivision_name"`
	DivisionID       string   `json:"division_id"`
	DivisionName     string   `json:"division_name"`
	BranchID         string   `json:"branch_id"`
	BranchName       string   `json:"branch_name"`
	Position         *string  `json:"position"`
	JoinDate         *string  `json:"join_date"`
	Status           string   `json:"status"`
	DefaultShiftID   *string  `json:"default_shift_id"`
	DefaultShiftName *string  `json:"default_shift_name"`
	FaceEnrolled     bool     `json:"face_enrolled"`
	HasPhoto         bool     `json:"has_photo"`
	PhotoUpdatedAt   *string  `json:"photo_updated_at"`
	BaseSalary       *string  `json:"base_salary,omitempty"`
	Account          *Account `json:"account,omitempty"`
	// Children hanya dikirim di detail untuk SDM dan super admin (D53).
	Children *[]Child `json:"children,omitempty"`
}

// Account adalah ringkasan akun, hanya untuk pemegang user:manage.
type Account struct {
	UserID     string `json:"user_id"`
	Role       string `json:"role"`
	IsActive   bool   `json:"is_active"`
	IsLocked   bool   `json:"is_locked"`
	MFAEnabled bool   `json:"mfa_enabled"`
}

// Created adalah respons pembuatan Pejuang, memuat kata sandi awal untuk disampaikan SDM (D53).
type Created struct {
	Employee
	auth.InitialCredential
}

// Input juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya. Children nil berarti
// data anak tidak diubah.
type Input struct {
	Nip            *string  `json:"nip"`
	FullName       string   `json:"full_name"`
	Email          *string  `json:"email"`
	Phone          *string  `json:"phone"`
	Gender         *string  `json:"gender"`
	MaritalStatus  *string  `json:"marital_status"`
	HasChildren    *bool    `json:"has_children"`
	ChildrenCount  *int     `json:"children_count"`
	Children       *[]Child `json:"children"`
	SubdivisionID  string   `json:"subdivision_id"`
	Position       *string  `json:"position"`
	JoinDate       *string  `json:"join_date"`
	BaseSalary     string   `json:"base_salary"`
	Status         string   `json:"status"`
	DefaultShiftID *string  `json:"default_shift_id"`
}

type validated struct {
	Nip            string
	FullName       string
	Email          pgtype.Text
	Phone          pgtype.Text
	Gender         pgtype.Text
	MaritalStatus  pgtype.Text
	HasChildren    pgtype.Bool
	ChildrenCount  pgtype.Int2
	Children       []store.InsertEmployeeChildParams
	SetChildren    bool
	SubdivisionID  uuid.UUID
	Position       pgtype.Text
	JoinDate       pgtype.Date
	BaseSalary     decimal.Decimal
	Status         string
	DefaultShiftID uuid.NullUUID
}

func (in Input) validate(today time.Time) (validated, error) {
	fields := map[string]string{}
	v := validated{
		FullName: strings.TrimSpace(in.FullName),
		Email:    pgconv.Text(in.Email),
		Position: pgconv.Text(in.Position),
		Status:   in.Status,
	}
	if v.Email.Valid {
		v.Email.String = strings.TrimSpace(v.Email.String)
	}
	if raw := pgconv.Text(in.Phone); raw.Valid {
		normalized, ok := phone.Normalize(raw.String)
		if !ok {
			fields["phone"] = "Mohon periksa kembali nomor WA, contoh 081234567890."
		}
		v.Phone = pgtype.Text{String: normalized, Valid: ok}
	}
	var msg string
	if v.Gender, msg = optionalEnum(in.Gender, Genders, "Jenis kelamin L atau P."); msg != "" {
		fields["gender"] = msg
	}
	if v.MaritalStatus, msg = optionalEnum(in.MaritalStatus, MaritalStatuses, "Mohon pilih status pernikahan dari daftar."); msg != "" {
		fields["marital_status"] = msg
	}
	if in.Children != nil {
		v.SetChildren = true
		v.Children = validateChildren(*in.Children, today, fields)
	}
	if in.HasChildren != nil {
		v.HasChildren = pgtype.Bool{Bool: *in.HasChildren, Valid: true}
	}
	if in.ChildrenCount != nil {
		if *in.ChildrenCount < 0 || *in.ChildrenCount > maxChildren {
			fields["children_count"] = fmt.Sprintf("Jumlah anak 0 sampai %d.", maxChildren)
		} else {
			v.ChildrenCount = pgtype.Int2{Int16: int16(*in.ChildrenCount), Valid: true}
		}
	}
	switch {
	case v.HasChildren.Valid && !v.HasChildren.Bool && ((v.ChildrenCount.Valid && v.ChildrenCount.Int16 > 0) || len(v.Children) > 0):
		fields["has_children"] = "Isi \"memiliki anak\" dengan Ya bila ada data anak."
	case v.ChildrenCount.Valid && int(v.ChildrenCount.Int16) < len(v.Children):
		fields["children_count"] = "Jumlah anak perlu sama atau lebih dari data anak yang diisi."
	}
	if in.Nip != nil {
		v.Nip = strings.ToUpper(strings.TrimSpace(*in.Nip))
		if v.Nip != "" && !nipPattern.MatchString(v.Nip) {
			fields["nip"] = "NIPY 4-20 karakter huruf, angka, titik, atau tanda minus."
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
	// Email tidak lagi wajib: login bisa memakai NIPY atau nomor WA (D53).
	if v.Email.Valid && (len(v.Email.String) > 254 || !emailPattern.MatchString(v.Email.String)) {
		fields["email"] = "Mohon periksa kembali format email, contoh nama@contoh.id."
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
			fields["join_date"] = "Mohon periksa kembali tanggal."
		}
		v.JoinDate = d
	}
	switch {
	case in.BaseSalary == "":
		fields["base_salary"] = "Ubar pokok wajib diisi."
	case !moneyPattern.MatchString(in.BaseSalary):
		fields["base_salary"] = "Ubar pokok diisi angka, contoh 4500000.00."
	default:
		amount, err := decimal.NewFromString(in.BaseSalary)
		if err != nil || amount.GreaterThan(maxBaseSalary) {
			fields["base_salary"] = "Ubar pokok melebihi batas maksimal."
		}
		v.BaseSalary = amount
	}
	if v.Status != "active" && v.Status != "inactive" {
		fields["status"] = "Mohon pilih status dari daftar."
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
	if strings.Contains(constraint, "phone") {
		return httpx.Duplicate("phone", "Nomor WA sudah dipakai Pejuang lain.")
	}
	return httpx.Duplicate("nip", "NIPY sudah dipakai Pejuang lain.")
}

// ListParams adalah filter resmi daftar Pejuang.
type ListParams struct {
	Q             string
	BranchID      uuid.NullUUID
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
		Q: q, BranchID: params.BranchID, DivisionID: params.DivisionID, SubdivisionID: params.SubdivisionID, Status: status,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListEmployees(ctx, store.ListEmployeesParams{
		ScopeAll: all, ScopeSubdivisionIds: subs, ScopeDivisionIds: divs,
		Q: q, BranchID: params.BranchID, DivisionID: params.DivisionID, SubdivisionID: params.SubdivisionID, Status: status,
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
	if seesChildren(p) || id == p.EmployeeID {
		children, err := s.ListChildren(ctx, id)
		if err != nil {
			return Employee{}, err
		}
		out[0].Children = &children
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
		return httpx.Validation(map[string]string{"subdivision_id": "Subdiv belum terdaftar."})
	}
	return err
}

func (s *Service) assertShift(ctx context.Context, id uuid.NullUUID) error {
	if !id.Valid {
		return nil
	}
	_, err := s.q.GetShift(ctx, id.UUID)
	if pgconv.IsNoRows(err) {
		return httpx.Validation(map[string]string{"default_shift_id": "Shift belum terdaftar."})
	}
	return err
}

// Create membuat data Pejuang dan akun aktif dengan kata sandi awal yang wajib diganti (D53).
func (s *Service) Create(ctx context.Context, p auth.Principal, ip audit.Entry, in Input, now time.Time) (Created, error) {
	// Selain SDM tidak mengurus ubar: Pejuang dibuat dengan ubar 0 dan SDM yang mengisinya (D34).
	if !permission.ManagesSalary(p.Role) {
		if in.BaseSalary != "" {
			return Created{}, httpx.ErrForbidden()
		}
		in.BaseSalary = "0.00"
	}
	if in.Status == "" {
		in.Status = "active"
	}
	v, err := in.validate(pgconv.WorkDate(now))
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
	credential, hash, err := auth.InitialPasswordHash(v.FullName, now)
	if err != nil {
		return Created{}, err
	}
	if !v.JoinDate.Valid {
		v.JoinDate = pgconv.Date(pgconv.WorkDate(now))
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		id, _, err = s.insert(ctx, q, ip, v, hash, credential.Expires(), now)
		return err
	})
	if err != nil {
		return Created{}, err
	}
	employee, err := s.Get(ctx, p, id)
	if err != nil {
		return Created{}, err
	}
	return Created{Employee: employee, InitialCredential: credential}, nil
}

// insert menulis data Pejuang baru, data anak, dan akunnya dalam transaksi pemanggil. Dipakai
// tambah satu per satu dan impor massal.
func (s *Service) insert(ctx context.Context, q *store.Queries, ip audit.Entry, v validated, passwordHash string, expires pgtype.Timestamptz, now time.Time) (uuid.UUID, string, error) {
	nip := v.Nip
	if nip == "" {
		seq, err := q.NextNipSequence(ctx)
		if err != nil {
			return uuid.Nil, "", err
		}
		nip = fmt.Sprintf("PJ%d%04d", now.In(pgconv.WIB).Year(), seq)
	}
	id, err := q.CreateEmployee(ctx, store.CreateEmployeeParams{
		Nip: nip, FullName: v.FullName, Email: v.Email, Phone: v.Phone,
		SubdivisionID: v.SubdivisionID, Position: v.Position, JoinDate: v.JoinDate,
		BaseSalary: v.BaseSalary, Status: v.Status, DefaultShiftID: v.DefaultShiftID,
		Gender: v.Gender, MaritalStatus: v.MaritalStatus, HasChildren: v.HasChildren, ChildrenCount: v.ChildrenCount,
	})
	if err != nil {
		return uuid.Nil, "", mapUnique(err)
	}
	if err := replaceChildren(ctx, q, id, v.Children); err != nil {
		return uuid.Nil, "", err
	}
	userID, err := q.CreateUserWithInitialPassword(ctx, store.CreateUserWithInitialPasswordParams{
		EmployeeID: id, Email: v.Email, PasswordHash: passwordHash, Role: store.UserRolePejuang,
		InitialPasswordExpiresAt: expires,
	})
	if err != nil {
		return uuid.Nil, "", mapUnique(err)
	}
	entry := ip
	entry.Action, entry.Entity, entry.EntityID = audit.EmployeeCreated, "employee", id
	if err := audit.Write(ctx, q, entry); err != nil {
		return uuid.Nil, "", err
	}
	entry.Action, entry.Entity, entry.EntityID = audit.UserCreated, "user", userID
	return id, nip, audit.Write(ctx, q, entry)
}

// currentInput mengisi Input dengan data tersimpan, agar PATCH hanya menimpa field yang dikirim.
func currentInput(current store.GetEmployeeScopedRow) Input {
	nip := current.Nip
	in := Input{
		Nip:            &nip,
		FullName:       current.FullName,
		Email:          pgconv.TextPtr(current.Email),
		Phone:          pgconv.TextPtr(current.Phone),
		Gender:         pgconv.TextPtr(current.Gender),
		MaritalStatus:  pgconv.TextPtr(current.MaritalStatus),
		SubdivisionID:  current.SubdivisionID.String(),
		Position:       pgconv.TextPtr(current.Position),
		JoinDate:       pgconv.DateString(current.JoinDate),
		BaseSalary:     current.BaseSalary.StringFixed(2),
		Status:         current.Status,
		DefaultShiftID: pgconv.UUIDPtr(current.DefaultShiftID),
	}
	if current.HasChildren.Valid {
		v := current.HasChildren.Bool
		in.HasChildren = &v
	}
	if current.ChildrenCount.Valid {
		v := int(current.ChildrenCount.Int16)
		in.ChildrenCount = &v
	}
	return in
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
	in := currentInput(current)
	if err := decode(&in); err != nil {
		return Employee{}, err
	}
	if !permission.ManagesSalary(p.Role) && in.BaseSalary != current.BaseSalary.StringFixed(2) {
		return Employee{}, httpx.ErrForbidden()
	}
	v, err := in.validate(pgconv.WorkDate(time.Now()))
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
		return update(ctx, q, ip, id, current, v)
	})
	if err != nil {
		return Employee{}, err
	}
	return s.Get(ctx, p, id)
}

// update menulis perubahan data Pejuang dalam transaksi pemanggil. Dipakai ubah satu per satu dan
// impor massal (NIPY yang sudah ada).
func update(ctx context.Context, q *store.Queries, ip audit.Entry, id uuid.UUID, current store.GetEmployeeScopedRow, v validated) error {
	affected, err := q.UpdateEmployee(ctx, store.UpdateEmployeeParams{
		ID: id, Nip: v.Nip, FullName: v.FullName, Email: v.Email, Phone: v.Phone,
		SubdivisionID: v.SubdivisionID, Position: v.Position, JoinDate: v.JoinDate,
		BaseSalary: v.BaseSalary, Status: v.Status, DefaultShiftID: v.DefaultShiftID,
		Gender: v.Gender, MaritalStatus: v.MaritalStatus, HasChildren: v.HasChildren, ChildrenCount: v.ChildrenCount,
	})
	if err != nil {
		return mapUnique(err)
	}
	if affected == 0 {
		return httpx.ErrNotFound()
	}
	if v.SetChildren {
		if err := replaceChildren(ctx, q, id, v.Children); err != nil {
			return err
		}
	}
	if v.Status == "inactive" || v.SubdivisionID != current.SubdivisionID {
		if err := q.ClearHeadOfEmployee(ctx, uuid.NullUUID{UUID: id, Valid: true}); err != nil {
			return err
		}
	}
	// Email login ikut berubah bersama email Pejuang agar keduanya selalu sama (D53).
	if v.Email != current.Email {
		if err := q.UpdateUserEmail(ctx, store.UpdateUserEmailParams{EmployeeID: id, Email: v.Email}); err != nil {
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
}

func errSelfDeactivate() error {
	return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Akun sendiri dinonaktifkan oleh pengelola lain.")
}

// Deactivate menonaktifkan Pejuang; riwayat presensi dan ubar tetap tersimpan.
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
		Gender: pgconv.TextPtr(r.Gender), MaritalStatus: pgconv.TextPtr(r.MaritalStatus),
		SubdivisionID: r.SubdivisionID.String(), SubdivisionName: r.SubdivisionName,
		DivisionID: r.DivisionID.String(), DivisionName: r.DivisionName,
		BranchID: r.BranchID.String(), BranchName: r.BranchName,
		Position: pgconv.TextPtr(r.Position), JoinDate: pgconv.DateString(r.JoinDate),
		Status:         r.Status,
		DefaultShiftID: pgconv.UUIDPtr(r.DefaultShiftID), DefaultShiftName: pgconv.TextPtr(r.DefaultShiftName),
		FaceEnrolled: r.FaceEnrolled,
		HasPhoto:     r.PhotoKey.Valid, PhotoUpdatedAt: pgconv.TimeString(r.PhotoUpdatedAt),
	}
	if permission.ManagesSalary(p.Role) || r.ID == p.EmployeeID {
		salary := r.BaseSalary.StringFixed(2)
		e.BaseSalary = &salary
	}
	if !seesChildren(p) && r.ID != p.EmployeeID {
		return e
	}
	if r.HasChildren.Valid {
		v := r.HasChildren.Bool
		e.HasChildren = &v
	}
	if r.ChildrenCount.Valid {
		v := int(r.ChildrenCount.Int16)
		e.ChildrenCount = &v
	}
	return e
}
