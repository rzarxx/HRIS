// Package settings menyimpan pengaturan aplikasi yang diatur super admin (settings:manage), dimulai
// dari footer web admin dan aplikasi.
package settings

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	keyFooter = "footer_template"
	// YearToken diganti tahun berjalan (WIB) saat footer ditampilkan.
	YearToken = "{tahun}"
	// DefaultFooter dipakai selama super admin belum mengubahnya.
	DefaultFooter = "© {tahun} PUTI (Pusat Teknologi Informasi) Al Bahjah"
	maxFooterLen  = 200
)

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool), now: time.Now}
}

// SetClock mengganti sumber waktu, untuk tes tahun otomatis.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Branding adalah teks merek yang tampil di web admin dan aplikasi.
type Branding struct {
	// Footer yang sudah jadi, dengan {tahun} diganti tahun berjalan.
	Footer string `json:"footer"`
	// Templat mentah; {tahun} diganti otomatis.
	FooterTemplate string  `json:"footer_template"`
	IsDefault      bool    `json:"is_default"`
	UpdatedAt      *string `json:"updated_at"`
}

// RenderFooter mengganti {tahun} dengan tahun pada [now] di zona WIB.
func RenderFooter(template string, now time.Time) string {
	return strings.ReplaceAll(template, YearToken, strconv.Itoa(now.In(pgconv.WIB).Year()))
}

// cleanFooter membuang karakter kontrol (termasuk baris baru) dan spasi berlebih; footer selalu teks murni.
func cleanFooter(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func (s *Service) Branding(ctx context.Context) (Branding, error) {
	row, err := s.q.GetAppSetting(ctx, keyFooter)
	out := Branding{FooterTemplate: DefaultFooter, IsDefault: true}
	switch {
	case pgconv.IsNoRows(err):
	case err != nil:
		return Branding{}, err
	default:
		out.FooterTemplate, out.IsDefault, out.UpdatedAt = row.Value, false, pgconv.TimeString(row.UpdatedAt)
	}
	out.Footer = RenderFooter(out.FooterTemplate, s.now())
	return out, nil
}

// FooterInput mengganti templat footer; null atau kosong mengembalikan teks bawaan.
type FooterInput struct {
	FooterTemplate *string `json:"footer_template"`
}

func (s *Service) SetFooter(ctx context.Context, entry audit.Entry, in FooterInput) (Branding, error) {
	value := ""
	if in.FooterTemplate != nil {
		value = cleanFooter(*in.FooterTemplate)
	}
	if utf8.RuneCountInString(value) > maxFooterLen {
		return Branding{}, httpx.Validation(map[string]string{"footer_template": "Footer maksimal 200 karakter."})
	}
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if value == "" || value == DefaultFooter {
			if _, err := q.DeleteAppSetting(ctx, keyFooter); err != nil {
				return err
			}
		} else if err := q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: keyFooter, Value: value, UpdatedBy: entry.UserID}); err != nil {
			return err
		}
		entry.Action, entry.Entity = audit.SettingsChanged, "settings"
		entry.Detail = map[string]any{"key": keyFooter, "reset": value == "" || value == DefaultFooter}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return Branding{}, err
	}
	return s.Branding(ctx)
}

// Handler melayani /v1/branding dan /v1/admin/settings/footer.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Branding melayani GET /v1/branding (tanpa login, untuk layar masuk) dan GET /v1/admin/settings/footer.
func (h *Handler) Branding(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Branding(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// SetFooter melayani PUT /v1/admin/settings/footer.
func (h *Handler) SetFooter(w http.ResponseWriter, r *http.Request) error {
	var in FooterInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.SetFooter(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
