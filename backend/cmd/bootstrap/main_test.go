package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/store"
)

type fakeBootstrapRepo struct {
	hasSDM       bool
	shifts       []store.Shift
	divisions    []store.ListDivisionsRow
	subdivisions []store.ListSubdivisionsRow
	createdEmp   *store.CreateEmployeeParams
	createdUser  *store.CreateUserParams
	audits       []store.InsertAuditLogParams
}

func (f *fakeBootstrapRepo) HasSDMUser(context.Context) (bool, error) {
	return f.hasSDM, nil
}

func (f *fakeBootstrapRepo) ListShifts(context.Context) ([]store.Shift, error) {
	return f.shifts, nil
}

func (f *fakeBootstrapRepo) InsertShift(_ context.Context, arg store.InsertShiftParams) (store.Shift, error) {
	s := store.Shift{ID: uuid.New(), Name: arg.Name}
	f.shifts = append(f.shifts, s)
	return s, nil
}

func (f *fakeBootstrapRepo) ListDivisions(context.Context) ([]store.ListDivisionsRow, error) {
	return f.divisions, nil
}

func (f *fakeBootstrapRepo) CreateDivision(_ context.Context, arg store.CreateDivisionParams) (store.Division, error) {
	id := uuid.New()
	f.divisions = append(f.divisions, store.ListDivisionsRow{ID: id, Code: arg.Code, Name: arg.Name})
	return store.Division{ID: id, Code: arg.Code, Name: arg.Name}, nil
}

func (f *fakeBootstrapRepo) ListSubdivisions(context.Context, uuid.NullUUID) ([]store.ListSubdivisionsRow, error) {
	return f.subdivisions, nil
}

func (f *fakeBootstrapRepo) CreateSubdivision(_ context.Context, arg store.CreateSubdivisionParams) (store.Subdivision, error) {
	id := uuid.New()
	f.subdivisions = append(f.subdivisions, store.ListSubdivisionsRow{ID: id, DivisionID: arg.DivisionID, Code: arg.Code, Name: arg.Name})
	return store.Subdivision{ID: id, DivisionID: arg.DivisionID, Code: arg.Code, Name: arg.Name}, nil
}

func (f *fakeBootstrapRepo) CreateEmployee(_ context.Context, arg store.CreateEmployeeParams) (uuid.UUID, error) {
	f.createdEmp = &arg
	return uuid.New(), nil
}

func (f *fakeBootstrapRepo) CreateUser(_ context.Context, arg store.CreateUserParams) (uuid.UUID, error) {
	f.createdUser = &arg
	return uuid.New(), nil
}

func (f *fakeBootstrapRepo) InsertAuditLog(_ context.Context, arg store.InsertAuditLogParams) error {
	f.audits = append(f.audits, arg)
	return nil
}

func TestValidateOptions(t *testing.T) {
	valid := options{
		Email:           "  Admin.SDM@AlBahjah.or.id ",
		FullName:        "Admin SDM",
		NIP:             "SDM0001",
		Position:        "Admin SDM",
		DivisionCode:    "SDM",
		DivisionName:    "Sumber Daya Manusia",
		SubdivisionCode: "SDM-PRS",
		SubdivisionName: "Personalia",
		Password:        "SandiProduksiMinimal12!",
	}
	if err := validateOptions(&valid, "BOOTSTRAP_SDM_PASSWORD"); err != nil {
		t.Fatalf("valid options gagal: %v", err)
	}
	if valid.Email != "admin.sdm@albahjah.or.id" {
		t.Errorf("email tidak dinormalisasi: %q", valid.Email)
	}

	short := valid
	short.Password = "terlalu-pdk"
	if err := validateOptions(&short, "BOOTSTRAP_SDM_PASSWORD"); err == nil {
		t.Fatal("sandi pendek seharusnya ditolak")
	}

	badEmail := valid
	badEmail.Email = "bukan-email"
	if err := validateOptions(&badEmail, "BOOTSTRAP_SDM_PASSWORD"); err == nil {
		t.Fatal("email tidak valid seharusnya ditolak")
	}
}

func TestBootstrapSDM(t *testing.T) {
	opts := options{
		Email:           "admin.sdm@albahjah.or.id",
		FullName:        "Admin SDM",
		NIP:             "SDM0001",
		Position:        "Admin SDM",
		DivisionCode:    "SDM",
		DivisionName:    "Sumber Daya Manusia",
		SubdivisionCode: "SDM-PRS",
		SubdivisionName: "Personalia",
		Password:        "SandiProduksiMinimal12!",
	}
	now := time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)

	t.Run("fresh database creates shift division subdivision employee sdm and audit", func(t *testing.T) {
		repo := &fakeBootstrapRepo{}
		res, err := bootstrapSDM(context.Background(), repo, opts, now)
		if err != nil {
			t.Fatalf("bootstrapSDM gagal: %v", err)
		}
		if res.UserID == uuid.Nil || res.EmployeeID == uuid.Nil || res.ShiftID == uuid.Nil {
			t.Fatalf("ID tidak boleh kosong: %+v", res)
		}
		if len(repo.shifts) != 1 || len(repo.divisions) != 1 || len(repo.subdivisions) != 1 {
			t.Fatalf("entitas awal tidak lengkap: shifts=%d div=%d sub=%d", len(repo.shifts), len(repo.divisions), len(repo.subdivisions))
		}
		if repo.createdUser == nil || repo.createdUser.Role != store.UserRoleSdm || !repo.createdUser.MustChangePassword {
			t.Fatalf("akun user tidak sesuai: %+v", repo.createdUser)
		}
		if len(repo.audits) != 1 || repo.audits[0].Action != "user.created" {
			t.Fatalf("audit log tidak sesuai: %+v", repo.audits)
		}
	})

	t.Run("rejects when sdm already exists", func(t *testing.T) {
		repo := &fakeBootstrapRepo{hasSDM: true}
		if _, err := bootstrapSDM(context.Background(), repo, opts, now); err == nil {
			t.Fatal("seharusnya menolak bootstrap jika SDM sudah ada")
		}
	})

	t.Run("reuses existing division subdivision and shift", func(t *testing.T) {
		shiftID := uuid.New()
		divID := uuid.New()
		subID := uuid.New()
		repo := &fakeBootstrapRepo{
			shifts:       []store.Shift{{ID: shiftID, Name: "Reguler"}},
			divisions:    []store.ListDivisionsRow{{ID: divID, Code: "SDM", Name: "Sumber Daya Manusia"}},
			subdivisions: []store.ListSubdivisionsRow{{ID: subID, DivisionID: divID, Code: "SDM-PRS", Name: "Personalia"}},
		}
		res, err := bootstrapSDM(context.Background(), repo, opts, now)
		if err != nil {
			t.Fatalf("bootstrapSDM gagal: %v", err)
		}
		if res.ShiftID != shiftID || res.DivisionID != divID || res.SubdivisionID != subID {
			t.Fatalf("tidak memakai data yang sudah ada: %+v", res)
		}
		if len(repo.shifts) != 1 || len(repo.divisions) != 1 || len(repo.subdivisions) != 1 {
			t.Fatalf("duplikat entitas dibuat")
		}
	})
}
