// Command bootstrap menyiapkan akun SDM pertama beserta divisi, subdivisi, dan shift awal di server baru.
// Aman dijalankan di APP_ENV=production karena tidak menyisipkan data dummy karyawan.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const minBootstrapPasswordLen = 12

type options struct {
	Email           string
	FullName        string
	NIP             string
	Position        string
	DivisionCode    string
	DivisionName    string
	SubdivisionCode string
	SubdivisionName string
	Password        string
}

type bootstrapRepo interface {
	HasSDMUser(ctx context.Context) (bool, error)
	ListShifts(ctx context.Context) ([]store.Shift, error)
	InsertShift(ctx context.Context, arg store.InsertShiftParams) (store.Shift, error)
	ListDivisions(ctx context.Context) ([]store.ListDivisionsRow, error)
	CreateDivision(ctx context.Context, arg store.CreateDivisionParams) (store.Division, error)
	ListSubdivisions(ctx context.Context, divisionID uuid.NullUUID) ([]store.ListSubdivisionsRow, error)
	CreateSubdivision(ctx context.Context, arg store.CreateSubdivisionParams) (store.Subdivision, error)
	CreateEmployee(ctx context.Context, arg store.CreateEmployeeParams) (uuid.UUID, error)
	CreateUser(ctx context.Context, arg store.CreateUserParams) (uuid.UUID, error)
	InsertAuditLog(ctx context.Context, arg store.InsertAuditLogParams) error
}

type result struct {
	UserID        uuid.UUID
	EmployeeID    uuid.UUID
	DivisionID    uuid.UUID
	SubdivisionID uuid.UUID
	ShiftID       uuid.UUID
	Email         string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gagal:", err)
		os.Exit(1)
	}
}

func run() error {
	var opts options
	var passwordEnv string

	flag.StringVar(&opts.Email, "email", "", "email akun SDM pertama (wajib)")
	flag.StringVar(&opts.FullName, "name", "Admin SDM", "nama lengkap admin SDM")
	flag.StringVar(&opts.NIP, "nip", "SDM0001", "NIP admin SDM")
	flag.StringVar(&opts.Position, "position", "Admin SDM", "jabatan")
	flag.StringVar(&opts.DivisionCode, "division-code", "SDM", "kode divisi awal")
	flag.StringVar(&opts.DivisionName, "division-name", "Sumber Daya Manusia", "nama divisi awal")
	flag.StringVar(&opts.SubdivisionCode, "subdivision-code", "SDM-PRS", "kode subdivisi awal")
	flag.StringVar(&opts.SubdivisionName, "subdivision-name", "Personalia", "nama subdivisi awal")
	flag.StringVar(&passwordEnv, "password-env", "BOOTSTRAP_SDM_PASSWORD", "nama env variable berisi sandi awal (minimal 12 karakter)")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	opts.Password = os.Getenv(passwordEnv)
	if err := validateOptions(&opts, passwordEnv); err != nil {
		return err
	}

	dsn := os.Getenv("MIGRATE_DATABASE_URL")
	if dsn == "" {
		return errors.New("MIGRATE_DATABASE_URL wajib diisi")
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	var res result
	if err := database.InTx(ctx, pool, func(q *store.Queries) error {
		var err error
		res, err = bootstrapSDM(ctx, q, opts, time.Now())
		return err
	}); err != nil {
		return err
	}

	fmt.Printf("Bootstrap SDM selesai.\n  Email      : %s\n  User ID    : %s\n  Employee ID: %s\n  Divisi/Sub : %s / %s\nMasuk ke panel web admin untuk mengganti sandi dan menyiapkan MFA TOTP.\n",
		res.Email, res.UserID, res.EmployeeID, opts.DivisionCode, opts.SubdivisionCode)
	return nil
}

func validateOptions(opts *options, passwordEnv string) error {
	opts.Email = strings.ToLower(strings.TrimSpace(opts.Email))
	opts.FullName = strings.TrimSpace(opts.FullName)
	opts.NIP = strings.TrimSpace(opts.NIP)
	opts.Position = strings.TrimSpace(opts.Position)
	opts.DivisionCode = strings.TrimSpace(opts.DivisionCode)
	opts.DivisionName = strings.TrimSpace(opts.DivisionName)
	opts.SubdivisionCode = strings.TrimSpace(opts.SubdivisionCode)
	opts.SubdivisionName = strings.TrimSpace(opts.SubdivisionName)

	if opts.Email == "" || !strings.Contains(opts.Email, "@") || strings.ContainsAny(opts.Email, " \t\r\n") {
		return errors.New("-email wajib berisi alamat email yang valid")
	}
	if opts.FullName == "" || opts.NIP == "" || opts.DivisionCode == "" || opts.DivisionName == "" || opts.SubdivisionCode == "" || opts.SubdivisionName == "" {
		return errors.New("nama, NIP, divisi, dan subdivisi tidak boleh kosong")
	}
	if len(opts.Password) < minBootstrapPasswordLen {
		return fmt.Errorf("%s minimal %d karakter", passwordEnv, minBootstrapPasswordLen)
	}
	return nil
}

func bootstrapSDM(ctx context.Context, repo bootstrapRepo, opts options, now time.Time) (result, error) {
	hasSDM, err := repo.HasSDMUser(ctx)
	if err != nil {
		return result{}, err
	}
	if hasSDM {
		return result{}, errors.New("akun SDM sudah ada; tambahkan pengguna lain melalui panel web admin")
	}

	shiftID, err := ensureShift(ctx, repo)
	if err != nil {
		return result{}, err
	}
	divisionID, err := ensureDivision(ctx, repo, opts.DivisionCode, opts.DivisionName)
	if err != nil {
		return result{}, err
	}
	subdivisionID, err := ensureSubdivision(ctx, repo, divisionID, opts.SubdivisionCode, opts.SubdivisionName)
	if err != nil {
		return result{}, err
	}

	empID, err := repo.CreateEmployee(ctx, store.CreateEmployeeParams{
		Nip:            opts.NIP,
		FullName:       opts.FullName,
		Email:          pgtype.Text{String: opts.Email, Valid: true},
		SubdivisionID:  subdivisionID,
		Position:       pgtype.Text{String: opts.Position, Valid: true},
		JoinDate:       pgconv.Date(now),
		BaseSalary:     decimal.Zero,
		Status:         "active",
		DefaultShiftID: uuid.NullUUID{UUID: shiftID, Valid: true},
	})
	if err != nil {
		return result{}, err
	}

	hash, err := auth.HashPassword(opts.Password)
	if err != nil {
		return result{}, err
	}
	userID, err := repo.CreateUser(ctx, store.CreateUserParams{
		EmployeeID:         empID,
		Email:              opts.Email,
		PasswordHash:       hash,
		Role:               store.UserRoleSdm,
		MustChangePassword: true,
	})
	if err != nil {
		return result{}, err
	}

	detail, err := json.Marshal(map[string]any{
		"bootstrap": true,
		"role":      "sdm",
		"email":     opts.Email,
		"nip":       opts.NIP,
	})
	if err != nil {
		return result{}, err
	}
	if err := repo.InsertAuditLog(ctx, store.InsertAuditLogParams{
		UserID:   audit.Actor(userID),
		Action:   audit.UserCreated,
		Entity:   pgtype.Text{String: "user", Valid: true},
		EntityID: uuid.NullUUID{UUID: userID, Valid: true},
		Detail:   detail,
	}); err != nil {
		return result{}, err
	}

	return result{
		UserID:        userID,
		EmployeeID:    empID,
		DivisionID:    divisionID,
		SubdivisionID: subdivisionID,
		ShiftID:       shiftID,
		Email:         opts.Email,
	}, nil
}

func ensureShift(ctx context.Context, repo bootstrapRepo) (uuid.UUID, error) {
	shifts, err := repo.ListShifts(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if len(shifts) > 0 {
		return shifts[0].ID, nil
	}
	created, err := repo.InsertShift(ctx, store.InsertShiftParams{
		Name:                 "Reguler",
		StartTime:            pgtype.Time{Microseconds: 8 * 3600 * 1_000_000, Valid: true},
		EndTime:              pgtype.Time{Microseconds: 17 * 3600 * 1_000_000, Valid: true},
		LateToleranceMinutes: 10,
		WorkDays:             []int16{1, 2, 3, 4, 5},
	})
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func ensureDivision(ctx context.Context, repo bootstrapRepo, code, name string) (uuid.UUID, error) {
	divisions, err := repo.ListDivisions(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	for _, d := range divisions {
		if strings.EqualFold(d.Code, code) {
			return d.ID, nil
		}
	}
	created, err := repo.CreateDivision(ctx, store.CreateDivisionParams{
		Code: code,
		Name: name,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func ensureSubdivision(ctx context.Context, repo bootstrapRepo, divisionID uuid.UUID, code, name string) (uuid.UUID, error) {
	subs, err := repo.ListSubdivisions(ctx, uuid.NullUUID{UUID: divisionID, Valid: true})
	if err != nil {
		return uuid.Nil, err
	}
	for _, s := range subs {
		if strings.EqualFold(s.Code, code) {
			return s.ID, nil
		}
	}
	created, err := repo.CreateSubdivision(ctx, store.CreateSubdivisionParams{
		DivisionID: divisionID,
		Code:       code,
		Name:       name,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}
