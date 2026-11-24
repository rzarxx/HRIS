package integration

import (
	"net/http"
	"strings"
	"testing"
)

type brandingRow struct {
	Footer         string  `json:"footer"`
	FooterTemplate string  `json:"footer_template"`
	IsDefault      bool    `json:"is_default"`
	UpdatedAt      *string `json:"updated_at"`
}

// TestFooterSetting menguji footer bawaan dengan tahun otomatis, pengubahan oleh super admin saja,
// dan pengembalian ke teks bawaan.
func TestFooterSetting(t *testing.T) {
	clock := &fakeClock{now: wibTime(2, 8, 0)}
	srv := fresh(t, options{clock: clock.Now})
	anon := newClient(t, srv)
	sdm := loginSDM(t, srv)
	super, _ := superadminClient(t, srv, "IT-DEV")

	def := data[brandingRow](t, anon.do(http.MethodGet, "/v1/branding", nil).want(t, http.StatusOK))
	if !def.IsDefault || def.Footer != "© 2026 PUTI (Pusat Teknologi Informasi) Al Bahjah" {
		t.Fatalf("bawaan = %+v", def)
	}

	sdm.do(http.MethodPut, "/v1/admin/settings/footer", map[string]any{"footer_template": "x"}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	super.do(http.MethodPut, "/v1/admin/settings/footer", map[string]any{"footer_template": strings.Repeat("a", 201)}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	set := data[brandingRow](t, super.do(http.MethodPut, "/v1/admin/settings/footer", map[string]any{
		"footer_template": "  PUTI Al Bahjah\n{tahun} ",
	}).want(t, http.StatusOK))
	if set.IsDefault || set.FooterTemplate != "PUTI Al Bahjah {tahun}" || set.Footer != "PUTI Al Bahjah 2026" || set.UpdatedAt == nil {
		t.Fatalf("setelah diubah = %+v", set)
	}
	clock.Set(wibTime(2, 8, 0).AddDate(1, 0, 0))
	if got := data[brandingRow](t, anon.do(http.MethodGet, "/v1/branding", nil).want(t, http.StatusOK)); got.Footer != "PUTI Al Bahjah 2027" {
		t.Errorf("tahun berganti otomatis = %q", got.Footer)
	}

	reset := data[brandingRow](t, super.do(http.MethodPut, "/v1/admin/settings/footer", map[string]any{"footer_template": nil}).want(t, http.StatusOK))
	if !reset.IsDefault || reset.Footer != "© 2027 PUTI (Pusat Teknologi Informasi) Al Bahjah" {
		t.Errorf("dikembalikan = %+v", reset)
	}
	if n := countAudit(t, "settings.changed"); n != 2 {
		t.Errorf("audit settings.changed = %d, ingin 2", n)
	}
}

type appVersionRow struct {
	LatestCode int     `json:"latest_code"`
	LatestName string  `json:"latest_name"`
	MinCode    int     `json:"min_code"`
	StoreURL   string  `json:"store_url"`
	Notes      string  `json:"notes"`
	UpdatedAt  *string `json:"updated_at"`
}

// TestAppVersionSetting menguji versi aplikasi (D63): bawaan tanpa login, hanya super admin yang mengubah,
// dan aplikasi di bawah versi minimum dijawab 426 kecuali endpoint versi.
func TestAppVersionSetting(t *testing.T) {
	srv := fresh(t, options{})
	anon := newClient(t, srv)
	sdm := loginSDM(t, srv)
	super, _ := superadminClient(t, srv, "IT-DEV")

	def := data[appVersionRow](t, anon.do(http.MethodGet, "/v1/app/version", nil).want(t, http.StatusOK))
	if def.LatestCode != 0 || def.MinCode != 0 || !strings.HasPrefix(def.StoreURL, "https://play.google.com/") || def.UpdatedAt != nil {
		t.Fatalf("bawaan = %+v", def)
	}

	body := map[string]any{"latest_code": 1400, "latest_name": "0.14.0", "min_code": 1300, "notes": "Tahan offline."}
	sdm.do(http.MethodPut, "/v1/admin/settings/app-version", body).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	super.do(http.MethodPut, "/v1/admin/settings/app-version", map[string]any{"latest_code": 1400, "latest_name": "0.14.0", "min_code": 1500}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	set := data[appVersionRow](t, super.do(http.MethodPut, "/v1/admin/settings/app-version", body).want(t, http.StatusOK))
	if set.LatestCode != 1400 || set.LatestName != "0.14.0" || set.MinCode != 1300 || set.Notes != "Tahan offline." || set.UpdatedAt == nil {
		t.Fatalf("setelah diubah = %+v", set)
	}

	old := newClient(t, srv).withHeaders(map[string]string{"X-App-Version": "0.12.0+1200"})
	old.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUpgradeRequired, "APP_UPDATE_REQUIRED")
	if got := data[appVersionRow](t, old.do(http.MethodGet, "/v1/app/version", nil).want(t, http.StatusOK)); got.LatestCode != 1400 {
		t.Errorf("versi lama tetap bisa membaca versi terbaru: %+v", got)
	}
	current := newClient(t, srv).withHeaders(map[string]string{"X-App-Version": "0.13.0+1300"})
	current.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	// Web tanpa header tidak terpengaruh.
	anon.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	if n := countAudit(t, "settings.changed"); n != 1 {
		t.Errorf("audit settings.changed = %d, ingin 1", n)
	}
}
