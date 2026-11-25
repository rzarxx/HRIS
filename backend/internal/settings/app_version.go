package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	keyAppVersion = "app_version"
	// DefaultStoreURL adalah halaman aplikasi di Play Store, dipakai bila super admin belum mengisi tautan lain.
	DefaultStoreURL = "https://play.google.com/store/apps/details?id=id.albahjah.hris"
	maxVersionCode  = 2_100_000_000
	maxNotesLen     = 500
	// minCodeTTL membatasi seberapa sering middleware membaca versi minimum dari database.
	minCodeTTL = time.Minute
)

var versionName = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// VersionCode menghitung versionCode dari nama versi x.y.z dengan rumus yang sama seperti
// .github/workflows/release.yml. False bila bentuknya salah atau minor/patch lebih dari 999.
func VersionCode(name string) (int, bool) {
	m := versionName.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	patch, err3 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil || err3 != nil || minor > 999 || patch > 999 || major > 2099 {
		return 0, false
	}
	return major*1_000_000 + minor*1_000 + patch, true
}

// AppVersion adalah versi aplikasi Android yang diumumkan ke Pejuang (D63). Kode versi sama dengan
// versionCode aplikasi dari workflow rilis (major·1.000.000 + minor·1.000 + patch, mis. 0.14.0 = 14000).
// Nol berarti belum diatur.
type AppVersion struct {
	LatestCode int     `json:"latest_code"`
	LatestName string  `json:"latest_name"`
	MinCode    int     `json:"min_code"`
	StoreURL   string  `json:"store_url"`
	Notes      string  `json:"notes"`
	UpdatedAt  *string `json:"updated_at"`
}

// AppVersionInput mengganti seluruh pengaturan versi aplikasi.
type AppVersionInput struct {
	LatestCode int    `json:"latest_code"`
	LatestName string `json:"latest_name"`
	MinCode    int    `json:"min_code"`
	StoreURL   string `json:"store_url"`
	Notes      string `json:"notes"`
}

// stored adalah bentuk JSON di app_settings; UpdatedAt diambil dari kolom tabel.
type storedAppVersion struct {
	LatestCode int    `json:"latest_code"`
	LatestName string `json:"latest_name"`
	MinCode    int    `json:"min_code"`
	StoreURL   string `json:"store_url"`
	Notes      string `json:"notes"`
}

// minCache menyimpan versi minimum sebentar agar pemeriksaan setiap permintaan tidak membebani database.
type minCache struct {
	mu      sync.Mutex
	code    int
	fetched time.Time
}

func (s *Service) AppVersion(ctx context.Context) (AppVersion, error) {
	out := AppVersion{StoreURL: DefaultStoreURL}
	row, err := s.q.GetAppSetting(ctx, keyAppVersion)
	switch {
	case pgconv.IsNoRows(err):
		return out, nil
	case err != nil:
		return AppVersion{}, err
	}
	var v storedAppVersion
	if err := json.Unmarshal([]byte(row.Value), &v); err != nil {
		return AppVersion{}, err
	}
	out.LatestCode, out.LatestName, out.MinCode, out.Notes = v.LatestCode, v.LatestName, v.MinCode, v.Notes
	if v.StoreURL != "" {
		out.StoreURL = v.StoreURL
	}
	out.UpdatedAt = pgconv.TimeString(row.UpdatedAt)
	return out, nil
}

// cleanNotes membuang karakter kontrol kecuali baris baru, dan merapikan spasi di ujung tiap baris.
func cleanNotes(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// validStoreURL menerima tautan https tanpa kredensial, mis. Play Store atau halaman unduh resmi.
func validStoreURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && !strings.ContainsAny(raw, " \"<>\\")
}

func normalizeAppVersion(in AppVersionInput) (storedAppVersion, error) {
	out := storedAppVersion{
		LatestCode: in.LatestCode,
		LatestName: strings.TrimSpace(in.LatestName),
		MinCode:    in.MinCode,
		StoreURL:   strings.TrimSpace(in.StoreURL),
		Notes:      cleanNotes(in.Notes),
	}
	fields := map[string]string{}
	if out.LatestCode < 0 || out.LatestCode > maxVersionCode {
		fields["latest_code"] = "Kode versi tidak valid."
	}
	if out.LatestCode > 0 {
		// Kode harus cocok dengan nama versi; salah hitung membuat pemberitahuan versi tidak pernah muncul.
		if code, ok := VersionCode(out.LatestName); !ok {
			fields["latest_name"] = "Nama versi harus berbentuk x.y.z, mis. 0.14.0."
		} else if code != out.LatestCode {
			fields["latest_code"] = "Kode versi tidak sesuai nama versi (0.14.0 = 14000)."
		}
	}
	if out.LatestCode == 0 && out.LatestName != "" {
		fields["latest_code"] = "Isi kode versi untuk nama versi ini."
	}
	if out.MinCode < 0 || out.MinCode > out.LatestCode {
		fields["min_code"] = "Versi minimum tidak boleh lebih tinggi dari versi terbaru."
	}
	if out.StoreURL == DefaultStoreURL {
		out.StoreURL = ""
	}
	if out.StoreURL != "" && !validStoreURL(out.StoreURL) {
		fields["store_url"] = "Tautan harus diawali https://."
	}
	if utf8.RuneCountInString(out.Notes) > maxNotesLen {
		fields["notes"] = "Catatan maksimal 500 karakter."
	}
	if len(fields) > 0 {
		return storedAppVersion{}, httpx.Validation(fields)
	}
	return out, nil
}

func (s *Service) SetAppVersion(ctx context.Context, entry audit.Entry, in AppVersionInput) (AppVersion, error) {
	v, err := normalizeAppVersion(in)
	if err != nil {
		return AppVersion{}, err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return AppVersion{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: keyAppVersion, Value: string(raw), UpdatedBy: entry.UserID}); err != nil {
			return err
		}
		entry.Action, entry.Entity = audit.SettingsChanged, "settings"
		entry.Detail = map[string]any{"key": keyAppVersion, "latest_code": v.LatestCode, "min_code": v.MinCode}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return AppVersion{}, err
	}
	s.min.mu.Lock()
	s.min.code, s.min.fetched = v.MinCode, s.now()
	s.min.mu.Unlock()
	return s.AppVersion(ctx)
}

// MinAppCode adalah versi minimum aplikasi yang masih dilayani, untuk middleware 426. Bila database gagal
// dibaca, aplikasi tetap dilayani (nol) agar gangguan sesaat tidak mengunci semua Pejuang.
func (s *Service) MinAppCode(ctx context.Context) int {
	s.min.mu.Lock()
	defer s.min.mu.Unlock()
	if !s.min.fetched.IsZero() && s.now().Sub(s.min.fetched) < minCodeTTL {
		return s.min.code
	}
	v, err := s.AppVersion(ctx)
	if err != nil {
		return s.min.code
	}
	s.min.code, s.min.fetched = v.MinCode, s.now()
	return s.min.code
}

// AppVersion melayani GET /v1/app/version (tanpa login) dan GET /v1/admin/settings/app-version.
func (h *Handler) AppVersion(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.AppVersion(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// SetAppVersion melayani PUT /v1/admin/settings/app-version.
func (h *Handler) SetAppVersion(w http.ResponseWriter, r *http.Request) error {
	var in AppVersionInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.SetAppVersion(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// MinAppCode menyediakan versi minimum untuk middleware server.
func (h *Handler) MinAppCode(ctx context.Context) int { return h.svc.MinAppCode(ctx) }
