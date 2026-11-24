package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAppVersion(t *testing.T) {
	cases := map[string]int{"0.14.0+1400": 1400, "1400": 1400, " 1.0.3+10003 ": 10003}
	for in, want := range cases {
		if got, ok := parseAppVersion(in); !ok || got != want {
			t.Errorf("%q = %d, %v; ingin %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "1.0.3", "abc", "0.1+0", "+-5"} {
		if _, ok := parseAppVersion(bad); ok {
			t.Errorf("%q seharusnya tidak terbaca", bad)
		}
	}
}

// Aplikasi di bawah versi minimum dijawab 426; web (tanpa header) dan endpoint versi tetap dilayani.
func TestRequireAppVersion(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := requireAppVersion(func(context.Context) int { return 1400 })(ok)
	send := func(path, version string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if version != "" {
			req.Header.Set(HeaderAppVersion, version)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := send("/v1/me", "0.13.0+1300"); got != http.StatusUpgradeRequired {
		t.Errorf("versi lama = %d, ingin 426", got)
	}
	if got := send("/v1/me", "0.14.0+1400"); got != http.StatusNoContent {
		t.Errorf("versi minimum = %d", got)
	}
	if got := send("/v1/me", ""); got != http.StatusNoContent {
		t.Errorf("tanpa header (web) = %d", got)
	}
	for _, open := range []string{"/v1/app/version", "/v1/auth/mobile/login", "/v1/auth/mobile/refresh", "/v1/auth/logout", "/v1/branding"} {
		if got := send(open, "0.1.0+100"); got != http.StatusNoContent {
			t.Errorf("%s untuk versi lama = %d", open, got)
		}
	}
	none := requireAppVersion(func(context.Context) int { return 0 })(ok)
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set(HeaderAppVersion, "0.1.0+100")
	rec := httptest.NewRecorder()
	none.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("minimum belum diatur = %d", rec.Code)
	}
}
