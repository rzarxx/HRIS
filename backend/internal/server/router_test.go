package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
)

const testWebOrigin = "http://localhost:3000"

var allRoles = []string{permission.RolePejuang, permission.RoleSubdiv, permission.RoleKadiv, permission.RoleSDM, permission.RoleSuperadmin}

func testRouter(t *testing.T, sessionActive bool) (http.Handler, *auth.TokenSigner, []Route) {
	t.Helper()
	return testRouterState(t, auth.SessionState{Active: sessionActive})
}

func testRouterState(t *testing.T, state auth.SessionState) (http.Handler, *auth.TokenSigner, []Route) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := auth.NewTokenSigner(private, "test")
	d := Deps{
		Signer: signer,
		SessionCheck: func(context.Context, uuid.UUID) (auth.SessionState, error) {
			return state, nil
		},
		Limiter: auth.NewMemoryLimiter(),
		Options: Options{AllowedOrigins: []string{testWebOrigin}},
	}
	// Handler asli diganti stub: tes ini hanya memeriksa gerbang akses.
	table := routes(d)
	for i := range table {
		table[i].Handler = func(w http.ResponseWriter, _ *http.Request) error {
			w.WriteHeader(http.StatusTeapot)
			return nil
		}
	}
	return build(d, table), signer, table
}

func call(h http.Handler, method, path, token, csrf string) *httptest.ResponseRecorder {
	path = strings.ReplaceAll(path, "{date}", "2026-08-17")
	req := httptest.NewRequest(method, strings.ReplaceAll(path, "{id}", uuid.NewString()), nil)
	// Tes ini memeriksa gerbang akses; penjaga akses langsung punya tes sendiri.
	req.Header.Set("Origin", testWebOrigin)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieAccess, Value: token})
	}
	if csrf != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieCSRF, Value: csrf})
		req.Header.Set(auth.HeaderCSRF, csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func tokenFor(t *testing.T, s *auth.TokenSigner, role string) string {
	t.Helper()
	token, err := s.Sign(uuid.New(), uuid.New(), uuid.New(), role, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body bukan error JSON: %q", rec.Body.String())
	}
	return body.Error.Code
}

// Matriks tiap role × tiap endpoint (MASTER.md §12.3).
func TestPermissionMatrix(t *testing.T) {
	h, signer, table := testRouter(t, true)
	for _, route := range table {
		for _, role := range allRoles {
			if route.Access == Public {
				continue
			}
			allowed := true
			for _, key := range route.Permissions {
				allowed = allowed && permission.Allows(role, key)
			}
			rec := call(h, route.Method, route.Path, tokenFor(t, signer, role), "csrf-ok")
			if allowed && rec.Code != http.StatusTeapot {
				t.Errorf("%s %s sebagai %s: ingin lolos, dapat %d", route.Method, route.Path, role, rec.Code)
			}
			if !allowed {
				if rec.Code != http.StatusForbidden || errorCode(t, rec) != "FORBIDDEN_ROLE" {
					t.Errorf("%s %s sebagai %s: ingin 403 FORBIDDEN_ROLE, dapat %d", route.Method, route.Path, role, rec.Code)
				}
			}
		}
	}
}

var publicPaths = map[string]bool{
	"/v1/auth/web/login":      true,
	"/v1/auth/web/refresh":    true,
	"/v1/auth/web/mfa/verify": true,
	"/v1/auth/activate":       true,
	"/v1/auth/activate/check": true,
}

// Saat sandi atau MFA belum dipenuhi, hanya route akun sendiri yang terbuka.
func TestGateBlocksGuardedRoutes(t *testing.T) {
	cases := []struct {
		state auth.SessionState
		code  string
	}{
		{auth.SessionState{Active: true, MustChangePassword: true}, "PASSWORD_CHANGE_REQUIRED"},
		{auth.SessionState{Active: true, MFASetupRequired: true}, "MFA_SETUP_REQUIRED"},
	}
	for _, c := range cases {
		h, signer, table := testRouterState(t, c.state)
		token := tokenFor(t, signer, permission.RoleSDM)
		for _, route := range table {
			rec := call(h, route.Method, route.Path, token, "csrf-ok")
			switch route.Access {
			case Guarded:
				if rec.Code != http.StatusForbidden || errorCode(t, rec) != c.code {
					t.Errorf("%s %s: ingin %s, dapat %d", route.Method, route.Path, c.code, rec.Code)
				}
			default:
				if rec.Code != http.StatusTeapot {
					t.Errorf("%s %s harus tetap terbuka, dapat %d", route.Method, route.Path, rec.Code)
				}
			}
		}
	}
}

func TestUserRateLimit(t *testing.T) {
	h, signer, _ := testRouter(t, true)
	token := tokenFor(t, signer, permission.RoleSDM)
	for i := 0; i < userLimitPerMinute; i++ {
		if rec := call(h, http.MethodGet, "/v1/me", token, ""); rec.Code != http.StatusTeapot {
			t.Fatalf("permintaan ke-%d: %d", i+1, rec.Code)
		}
	}
	rec := call(h, http.MethodGet, "/v1/me", token, "")
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "RATE_LIMITED" {
		t.Fatalf("ingin 429 RATE_LIMITED, dapat %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After harus ada")
	}
	if other := call(h, http.MethodGet, "/v1/me", tokenFor(t, signer, permission.RoleSDM), ""); other.Code != http.StatusTeapot {
		t.Errorf("user lain tidak boleh terpengaruh, dapat %d", other.Code)
	}
}

func TestPublicRoutesAreOnlyAllowlisted(t *testing.T) {
	_, _, table := testRouter(t, true)
	for _, route := range table {
		if route.Access == Public && !publicPaths[route.Path] {
			t.Errorf("route publik tak terduga: %s %s", route.Method, route.Path)
		}
		if route.Access == Guarded && len(route.Permissions) == 0 {
			t.Errorf("route tanpa permission: %s %s", route.Method, route.Path)
		}
	}
}

func TestNoTokenIsRejected(t *testing.T) {
	h, _, table := testRouter(t, true)
	for _, route := range table {
		if route.Access == Public {
			continue
		}
		rec := call(h, route.Method, route.Path, "", "csrf-ok")
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "AUTH_TOKEN_INVALID" {
			t.Errorf("%s %s tanpa token: dapat %d", route.Method, route.Path, rec.Code)
		}
	}
}

func TestRevokedSessionIsRejected(t *testing.T) {
	h, signer, _ := testRouter(t, false)
	rec := call(h, http.MethodGet, "/v1/me", tokenFor(t, signer, permission.RoleSDM), "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sesi dicabut: dapat %d", rec.Code)
	}
}

func TestTokenFromOtherKeyIsRejected(t *testing.T) {
	h, _, _ := testRouter(t, true)
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	token, err := auth.NewTokenSigner(other, "lain").Sign(uuid.New(), uuid.New(), uuid.New(), permission.RoleSDM, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec := call(h, http.MethodGet, "/v1/me", token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("token kunci lain: dapat %d", rec.Code)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	h, signer, _ := testRouter(t, true)
	token, err := signer.Sign(uuid.New(), uuid.New(), uuid.New(), permission.RoleSDM, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if rec := call(h, http.MethodGet, "/v1/me", token, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("token kedaluwarsa: dapat %d", rec.Code)
	}
}

func TestCSRFRequiredOnWrites(t *testing.T) {
	h, signer, _ := testRouter(t, true)
	token := tokenFor(t, signer, permission.RoleSDM)
	rec := call(h, http.MethodPost, "/v1/admin/divisions", token, "")
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "CSRF_INVALID" {
		t.Errorf("POST tanpa CSRF: dapat %d", rec.Code)
	}
	// Header berbeda dari cookie juga ditolak.
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/divisions", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccess, Value: token})
	req.AddCookie(&http.Cookie{Name: auth.CookieCSRF, Value: "a"})
	req.Header.Set(auth.HeaderCSRF, "b")
	mismatch := httptest.NewRecorder()
	h.ServeHTTP(mismatch, req)
	if mismatch.Code != http.StatusForbidden {
		t.Errorf("CSRF beda: dapat %d", mismatch.Code)
	}
	if rec := call(h, http.MethodGet, "/v1/me", token, ""); rec.Code != http.StatusTeapot {
		t.Errorf("GET tidak perlu CSRF: dapat %d", rec.Code)
	}
}

func TestRealIP(t *testing.T) {
	cases := []struct {
		name, remote, header string
		trust                bool
		want                 string
	}{
		{"proxy lokal dipercaya", "127.0.0.1:5000", "203.0.113.9", true, "203.0.113.9"},
		{"proxy lokal IPv6", "[::1]:5000", "2001:db8::1", true, "2001:db8::1"},
		{"klien luar tidak bisa memalsukan", "198.51.100.7:5000", "203.0.113.9", true, "198.51.100.7"},
		{"TRUST_PROXY mati", "127.0.0.1:5000", "203.0.113.9", false, "127.0.0.1"},
		{"header bukan IP", "127.0.0.1:5000", "bukan-ip", true, "127.0.0.1"},
		{"header kosong", "127.0.0.1:5000", "", true, "127.0.0.1"},
	}
	for _, c := range cases {
		var got string
		h := realIP(c.trust)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			if addr := audit.ClientIP(r); addr != nil {
				got = addr.String()
			}
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if c.header != "" {
			req.Header.Set("X-Real-IP", c.header)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if got != c.want {
			t.Errorf("%s: dapat %q, ingin %q", c.name, got, c.want)
		}
	}
}

func TestCORSOnlyAllowsConfiguredOrigin(t *testing.T) {
	h, _, _ := testRouter(t, true)
	preflight := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, "/v1/me", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if got := preflight("http://localhost:3000").Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("origin sah: %q", got)
	}
	if got := preflight("https://jahat.example").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("origin asing diizinkan: %q", got)
	}
}

func directRequest(method, path string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(path, "{id}", uuid.NewString()), "{date}", "2026-08-17"), "{user_id}", uuid.NewString()), "{session_id}", uuid.NewString()), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// Akses langsung dari luar tidak boleh mencapai route mana pun, termasuk route publik (D37).
func TestDirectAccessIsBlocked(t *testing.T) {
	h, _, table := testRouter(t, true)
	paths := []struct{ method, path string }{{http.MethodGet, "/"}, {http.MethodGet, "/healthz"}, {http.MethodGet, "/readyz"}, {http.MethodGet, "/tidak-ada"}, {http.MethodGet, "/.env"}}
	for _, r := range table {
		paths = append(paths, struct{ method, path string }{r.Method, r.Path})
	}
	for _, p := range paths {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, directRequest(p.method, p.path, nil))
		if rec.Code != http.StatusForbidden || errorCode(t, rec) != "API_ACCESS_DENIED" {
			t.Errorf("%s %s: %d %s", p.method, p.path, rec.Code, rec.Body.String())
		}
	}

	// Asal yang tidak dikenal, termasuk preflight, juga ditolak tanpa header CORS.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, directRequest(http.MethodOptions, "/v1/me", map[string]string{"Origin": "https://evil.example", "Access-Control-Request-Method": "GET"}))
	if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("preflight asing: %d %v", rec.Code, rec.Header())
	}
	// Respons penolakan tetap memuat header keamanan.
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("header keamanan hilang: %v", rec.Header())
	}
}

// Membuka alamat API dari browser dialihkan ke web admin, dan tujuannya tidak bisa dipengaruhi request.
func TestBrowserNavigationRedirectsToWeb(t *testing.T) {
	h, _, _ := testRouter(t, true)
	for name, headers := range map[string]map[string]string{
		"sec-fetch":  {"Sec-Fetch-Mode": "navigate"},
		"accept":     {"Accept": "text/html,application/xhtml+xml"},
		"host palsu": {"Accept": "text/html", "X-Forwarded-Host": "evil.example", "Referer": "https://evil.example/"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, directRequest(http.MethodGet, "/v1/admin/employees?next=https://evil.example", headers))
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != testWebOrigin+"/" {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("Location"))
		}
		if strings.Contains(rec.Body.String(), "employee") {
			t.Errorf("%s: isi respons bocor: %s", name, rec.Body.String())
		}
	}
	// Selain GET dan HEAD tidak pernah dialihkan.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, directRequest(http.MethodPost, "/v1/auth/web/login", map[string]string{"Accept": "text/html"}))
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST: %d", rec.Code)
	}
}

// Web admin dan aplikasi tetap bisa masuk, tetapi route data tetap wajib login.
func TestAllowedClientsStillNeedLogin(t *testing.T) {
	h, _, table := testRouter(t, true)
	for _, r := range table {
		if r.Access == Public {
			continue
		}
		for name, headers := range map[string]map[string]string{
			"web":      {"Origin": testWebOrigin},
			"aplikasi": {"Authorization": "Bearer palsu"},
		} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, directRequest(r.Method, r.Path, headers))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s (%s): %d %s", r.Method, r.Path, name, rec.Code, rec.Body.String())
			}
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, directRequest(http.MethodOptions, "/v1/me", map[string]string{"Origin": testWebOrigin, "Access-Control-Request-Method": "GET"}))
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != testWebOrigin {
		t.Errorf("preflight web: %d %v", rec.Code, rec.Header())
	}
}

// Pemeriksaan deploy dari mesin ini tetap jalan, tetapi request lewat Nginx tidak ikut dianggap lokal.
func TestLocalAccess(t *testing.T) {
	h, _, _ := testRouter(t, true)
	local := directRequest(http.MethodGet, "/readyz", nil)
	local.RemoteAddr = "127.0.0.1:50000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, local)
	if rec.Code != http.StatusOK {
		t.Errorf("lokal: %d", rec.Code)
	}
	for _, realIP := range []string{"203.0.113.9", "127.0.0.1"} {
		proxied := directRequest(http.MethodGet, "/readyz", map[string]string{"X-Real-IP": realIP})
		proxied.RemoteAddr = "127.0.0.1:50000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, proxied)
		if rec.Code != http.StatusForbidden {
			t.Errorf("lewat proxy %s: %d", realIP, rec.Code)
		}
	}
}
