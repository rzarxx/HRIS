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
