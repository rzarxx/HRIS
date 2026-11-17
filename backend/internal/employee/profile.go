package employee

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Nilai resmi formulir data Pejuang yayasan (D53, MASTER.md §6.2).
var (
	Genders         = []string{"L", "P"}
	MaritalStatuses = []string{"belum_menikah", "menikah", "cerai_hidup", "cerai_mati"}
	ChildRelations  = []string{"kandung", "angkat", "tiri"}
	EducationLevels = []string{"belum_sekolah", "paud", "tk_ra", "sd_mi", "smp_mts", "sma_sederajat", "perguruan_tinggi", "pondok_pesantren"}
)

const (
	maxChildren     = 30
	maxChildrenSent = 10
)

// Child adalah data satu anak Pejuang. Hanya terlihat oleh SDM, super admin, dan Pejuang sendiri.
type Child struct {
	ChildOrder int     `json:"child_order"`
	FullName   string  `json:"full_name"`
	Gender     *string `json:"gender"`
	BirthPlace *string `json:"birth_place"`
	BirthDate  *string `json:"birth_date"`
	Relation   *string `json:"relation"`
	Education  *string `json:"education"`
	SchoolName *string `json:"school_name"`
}

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// optionalEnum mengembalikan pgtype.Text kosong untuk nil/"" dan pesan galat untuk nilai di luar daftar.
func optionalEnum(v *string, allowed []string, message string) (pgtype.Text, string) {
	t := pgconv.Text(v)
	if !t.Valid {
		return t, ""
	}
	t.String = strings.TrimSpace(t.String)
	if !oneOf(t.String, allowed) {
		return pgtype.Text{}, message
	}
	return t, ""
}

func optionalLimited(v *string, max int, field, label string, fields map[string]string) pgtype.Text {
	t := pgconv.Text(v)
	if t.Valid {
		t.String = strings.TrimSpace(t.String)
		if len([]rune(t.String)) > max {
			fields[field] = fmt.Sprintf("%s maksimal %d karakter.", label, max)
		}
	}
	return t
}

// validateChildren memeriksa daftar anak; urutan anak (child_order) unik dan nama wajib diisi.
func validateChildren(children []Child, today time.Time, fields map[string]string) []store.InsertEmployeeChildParams {
	if len(children) > maxChildrenSent {
		fields["children"] = fmt.Sprintf("Maksimal %d anak per pengiriman.", maxChildrenSent)
		return nil
	}
	seen := map[int]bool{}
	out := make([]store.InsertEmployeeChildParams, 0, len(children))
	for i, c := range children {
		prefix := fmt.Sprintf("children.%d.", i)
		row := store.InsertEmployeeChildParams{ChildOrder: int16(c.ChildOrder), FullName: strings.TrimSpace(c.FullName)}
		switch {
		case c.ChildOrder < 1 || c.ChildOrder > maxChildren:
			fields[prefix+"child_order"] = fmt.Sprintf("Anak ke- 1 sampai %d.", maxChildren)
		case seen[c.ChildOrder]:
			fields[prefix+"child_order"] = "Urutan anak perlu berbeda satu sama lain."
		}
		seen[c.ChildOrder] = true
		if n := len([]rune(row.FullName)); n < 2 || n > 100 {
			fields[prefix+"full_name"] = "Nama anak 2 sampai 100 karakter."
		}
		var msg string
		if row.Gender, msg = optionalEnum(c.Gender, Genders, "Jenis kelamin L atau P."); msg != "" {
			fields[prefix+"gender"] = msg
		}
		if row.Relation, msg = optionalEnum(c.Relation, ChildRelations, "Mohon pilih status anak dari daftar."); msg != "" {
			fields[prefix+"relation"] = msg
		}
		if row.Education, msg = optionalEnum(c.Education, EducationLevels, "Mohon pilih jenjang pendidikan dari daftar."); msg != "" {
			fields[prefix+"education"] = msg
		}
		row.BirthPlace = optionalLimited(c.BirthPlace, 80, prefix+"birth_place", "Tempat lahir", fields)
		row.SchoolName = optionalLimited(c.SchoolName, 120, prefix+"school_name", "Nama sekolah", fields)
		if c.BirthDate != nil && strings.TrimSpace(*c.BirthDate) != "" {
			d, err := pgconv.ParseDate(strings.TrimSpace(*c.BirthDate))
			switch {
			case err != nil:
				fields[prefix+"birth_date"] = "Mohon periksa kembali tanggal lahir."
			case d.Time.After(today):
				fields[prefix+"birth_date"] = "Tanggal lahir perlu sebelum hari ini."
			}
			row.BirthDate = d
		}
		out = append(out, row)
	}
	return out
}

// seesChildren menjawab apakah pemanggil boleh melihat data anak Pejuang lain: hanya peran
// yang membaca seluruh Pejuang (SDM dan super admin). Atasan tidak (D53).
func seesChildren(p auth.Principal) bool {
	return permission.ScopeOf(p.Role, permission.EmployeeRead) == permission.ScopeAll
}

func presentChildren(rows []store.EmployeeChild) []Child {
	out := make([]Child, 0, len(rows))
	for _, r := range rows {
		out = append(out, Child{
			ChildOrder: int(r.ChildOrder), FullName: r.FullName,
			Gender: pgconv.TextPtr(r.Gender), BirthPlace: pgconv.TextPtr(r.BirthPlace), BirthDate: pgconv.DateString(r.BirthDate),
			Relation: pgconv.TextPtr(r.Relation), Education: pgconv.TextPtr(r.Education), SchoolName: pgconv.TextPtr(r.SchoolName),
		})
	}
	return out
}

// replaceChildren mengganti seluruh data anak seorang Pejuang dalam transaksi pemanggil.
func replaceChildren(ctx context.Context, q *store.Queries, employeeID uuid.UUID, children []store.InsertEmployeeChildParams) error {
	if err := q.DeleteEmployeeChildren(ctx, employeeID); err != nil {
		return err
	}
	for _, c := range children {
		c.EmployeeID = employeeID
		if err := q.InsertEmployeeChild(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// ListChildren dipakai profil Pejuang sendiri dan detail Pejuang untuk SDM.
func (s *Service) ListChildren(ctx context.Context, employeeID uuid.UUID) ([]Child, error) {
	rows, err := s.q.ListEmployeeChildren(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	return presentChildren(rows), nil
}
