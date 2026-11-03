package devsim

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// SeedSDMEmail adalah akun SDM data dummy.
const SeedSDMEmail = "sdm@pejuang.id"

type seedSub struct {
	code, name string
	size       int
	positions  []string
	salaryMin  int64
	salaryMax  int64
}

type seedDiv struct {
	code, name string
	subs       []seedSub
}

var seedOrg = []seedDiv{
	{"OPS", "Operasional", []seedSub{
		{"OPS-LAP", "Lapangan", 12, []string{"Staf Lapangan", "Koordinator Lapangan"}, 3_800_000, 5_200_000},
		{"OPS-GDG", "Gudang", 8, []string{"Staf Gudang", "Admin Gudang"}, 3_600_000, 4_800_000},
		{"OPS-DST", "Distribusi", 6, []string{"Kurir", "Pengemudi", "Admin Distribusi"}, 3_700_000, 5_000_000},
		{"OPS-KAM", "Keamanan", 5, []string{"Petugas Keamanan"}, 3_500_000, 4_200_000},
	}},
	{"KEU", "Keuangan", []seedSub{
		{"KEU-AKT", "Akuntansi", 5, []string{"Staf Akuntansi", "Akuntan"}, 5_000_000, 7_500_000},
		{"KEU-PJK", "Pajak", 3, []string{"Staf Pajak"}, 5_200_000, 7_000_000},
		{"KEU-TGH", "Penagihan", 4, []string{"Staf Penagihan", "Kolektor"}, 4_200_000, 6_000_000},
	}},
	{"IT", "Teknologi Informasi", []seedSub{
		{"IT-DEV", "Pengembangan", 6, []string{"Programmer", "QA Engineer", "UI/UX Designer"}, 6_500_000, 9_500_000},
		{"IT-INF", "Infrastruktur", 3, []string{"Network Engineer", "IT Support"}, 5_500_000, 8_000_000},
	}},
	{"SDM", "Sumber Daya Manusia", []seedSub{
		{"SDM-PRS", "Personalia", 3, []string{"Staf Personalia"}, 5_000_000, 7_000_000},
		{"SDM-UMM", "Umum", 3, []string{"Staf Umum", "Resepsionis"}, 3_800_000, 5_000_000},
	}},
}

var (
	firstNames = strings.Fields("Agus Budi Citra Dewi Eko Fajar Gita Hendra Indah Joko Kartika Lukman Maya Nanda Oki Putri Rizky Sari Taufik Utami Wahyu Yuni Zainal Ayu Bayu Dimas Endah Fitri Galih Hana Irfan Jihan Kurnia Lestari Rahmat Nur Andi Rina Teguh Wulan Arif Dian Siti Yoga Ratna Bagus Novi Ilham")
	lastNames  = strings.Fields("Santoso Wijaya Pratama Saputra Lestari Hidayat Kusuma Nugroho Rahayu Setiawan Siregar Permana Ramadhan Hakim Purnomo Maharani Syahputra Wibowo Anggraini Firmansyah")
)

// IsSeeded menjawab apakah database sudah berisi data.
func IsSeeded(ctx context.Context, q *store.Queries) (bool, error) {
	summary, err := q.DashboardSummary(ctx, pgconv.Date(pgconv.WorkDate(time.Now())))
	if err != nil {
		return false, err
	}
	return summary.DivisionCount > 0, nil
}

// Reset mengosongkan semua tabel data. Hanya untuk pengembangan; butuh role hris_migrate.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `TRUNCATE audit_logs, user_invites, user_mfa, employee_locations, office_locations, holidays, announcements, attendances, sessions, users, employees, subdivisions, divisions, shifts RESTART IDENTITY CASCADE`)
	return err
}

// Seed mengisi data dummy realistis: 4 divisi, 11 subdiv, 58 Pejuang, pengumuman, satu akun SDM.
func Seed(ctx context.Context, pool *pgxpool.Pool, sdmPassword string, now time.Time) error {
	if len(sdmPassword) < 10 {
		return fmt.Errorf("SEED_SDM_PASSWORD minimal 10 karakter")
	}
	hash, err := auth.HashPassword(sdmPassword)
	if err != nil {
		return err
	}
	return database.InTx(ctx, pool, func(q *store.Queries) error {
		return seedAll(ctx, q, hash, now)
	})
}

func seedAll(ctx context.Context, q *store.Queries, passwordHash string, now time.Time) error {
	r := rand.New(rand.NewSource(20261102))
	// Akun selain SDM belum aktif; hash acak yang sama cukup untuk data dummy.
	unusable, err := auth.UnusablePasswordHash()
	if err != nil {
		return err
	}
	shiftID, err := q.CreateShift(ctx, store.CreateShiftParams{
		Name:                 "Reguler",
		StartTime:            pgtype.Time{Microseconds: 8 * 3600 * 1_000_000, Valid: true},
		EndTime:              pgtype.Time{Microseconds: 17 * 3600 * 1_000_000, Valid: true},
		LateToleranceMinutes: 10,
	})
	if err != nil {
		return err
	}
	_ = shiftID

	used := map[string]bool{}
	seq := 1
	divisionIDs := map[string]uuid.UUID{}
	var sdmEmployee uuid.UUID
	var inactiveCandidate uuid.UUID

	for _, div := range seedOrg {
		division, err := q.CreateDivision(ctx, store.CreateDivisionParams{Code: div.code, Name: div.name})
		if err != nil {
			return err
		}
		divisionIDs[div.code] = division.ID
		var divisionHead uuid.UUID

		for subIdx, sub := range div.subs {
			subdivision, err := q.CreateSubdivision(ctx, store.CreateSubdivisionParams{DivisionID: division.ID, Code: sub.code, Name: sub.name})
			if err != nil {
				return err
			}
			for i := 0; i < sub.size; i++ {
				name := uniqueName(r, used)
				year := 2017 + r.Intn(9)
				salary := int64(float64(sub.salaryMin)+r.Float64()*float64(sub.salaryMax-sub.salaryMin)) / 50_000 * 50_000
				position := sub.positions[r.Intn(len(sub.positions))]
				isHead := i == 0
				isDivisionHead := isHead && subIdx == 0
				switch {
				case isDivisionHead:
					position = "Kepala Divisi " + div.name
					salary += 2_500_000
				case isHead:
					position = "Kepala Subdiv " + sub.name
					salary += 1_500_000
				}
				if sub.code == "SDM-PRS" && i == 1 {
					position = "Admin SDM"
				}
				id, err := q.CreateEmployee(ctx, store.CreateEmployeeParams{
					Nip:           fmt.Sprintf("PJ%d%04d", year, seq),
					FullName:      name,
					Email:         pgtype.Text{String: emailOf(name), Valid: true},
					Phone:         pgtype.Text{String: fmt.Sprintf("08%010d", 1_000_000_000+r.Int63n(8_999_999_999)), Valid: true},
					SubdivisionID: subdivision.ID,
					Position:      pgtype.Text{String: position, Valid: true},
					JoinDate:      pgconv.Date(time.Date(year, time.Month(1+r.Intn(12)), 1+r.Intn(27), 0, 0, 0, 0, time.UTC)),
					BaseSalary:    decimal.NewFromInt(salary),
					Status:        "active",
				})
				if err != nil {
					return err
				}
				seq++
				if !(sub.code == "SDM-PRS" && i == 1) {
					role := store.UserRolePejuang
					switch {
					case isDivisionHead:
						role = store.UserRoleKadiv
					case isHead:
						role = store.UserRoleSubdiv
					}
					if _, err := q.CreateInactiveUser(ctx, store.CreateInactiveUserParams{
						EmployeeID: id, Email: emailOf(name), PasswordHash: unusable, Role: role,
					}); err != nil {
						return err
					}
				}
				if isHead {
					if err := q.SetSubdivisionHead(ctx, store.SetSubdivisionHeadParams{ID: subdivision.ID, HeadEmployeeID: uuid.NullUUID{UUID: id, Valid: true}}); err != nil {
						return err
					}
				}
				if isDivisionHead {
					divisionHead = id
				}
				if sub.code == "SDM-PRS" && i == 1 {
					sdmEmployee = id
				}
				if sub.code == "SDM-UMM" && i == 2 {
					inactiveCandidate = id
				}
			}
		}
		if err := q.SetDivisionHead(ctx, store.SetDivisionHeadParams{ID: division.ID, HeadEmployeeID: uuid.NullUUID{UUID: divisionHead, Valid: true}}); err != nil {
			return err
		}
	}

	// Satu Pejuang nonaktif sebagai contoh status.
	if _, err := q.DeactivateEmployee(ctx, store.DeactivateEmployeeParams{ID: inactiveCandidate, ExitDate: pgconv.Date(AttendanceStart)}); err != nil {
		return err
	}

	// must_change_password=false hanya untuk data dummy: ganti sandi hadir di Fase 1.
	userID, err := q.CreateUser(ctx, store.CreateUserParams{
		EmployeeID: sdmEmployee, Email: SeedSDMEmail, PasswordHash: passwordHash,
		Role: store.UserRoleSdm, MustChangePassword: false,
	})
	if err != nil {
		return err
	}

	creator := uuid.NullUUID{UUID: userID, Valid: true}
	announcements := []store.CreateAnnouncementParams{
		{
			Title: "Selamat datang di HRIS Pejuang",
			Body:  "Mulai bulan ini presensi, pengajuan cuti, dan slip upah dapat diakses lewat aplikasi HRIS Pejuang.",
			Type:  store.AnnTypePopup, StartsAt: pgconv.Time(now.AddDate(0, 0, -5)), EndsAt: pgconv.Time(now.AddDate(0, 1, 0)),
		},
		{
			Title: "Briefing keselamatan kerja",
			Body:  "Seluruh Pejuang divisi Operasional wajib mengikuti briefing K3 setiap Senin pukul 07.45 WIB di halaman gudang.",
			Type:  store.AnnTypeBanner, DivisionID: uuid.NullUUID{UUID: divisionIDs["OPS"], Valid: true}, StartsAt: pgconv.Time(now.AddDate(0, 0, -12)),
		},
		{
			Title: "Upacara Hari Pahlawan",
			Body:  "Tanggal 10 November tetap masuk kerja seperti biasa. Upacara singkat pukul 08.00 WIB.",
			Type:  store.AnnTypeBanner, StartsAt: pgconv.Time(now.AddDate(0, 0, 3)), EndsAt: pgconv.Time(now.AddDate(0, 0, 8)),
		},
	}
	for _, a := range announcements {
		a.IsActive = true
		a.CreatedBy = creator
		if _, err := q.CreateAnnouncement(ctx, a); err != nil {
			return err
		}
	}
	_, err = FillAttendance(ctx, q, now)
	return err
}

func uniqueName(r *rand.Rand, used map[string]bool) string {
	for {
		name := firstNames[r.Intn(len(firstNames))] + " " + lastNames[r.Intn(len(lastNames))]
		if !used[name] {
			used[name] = true
			return name
		}
	}
}

func emailOf(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "@pejuang.id"
}
