package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/auth"
)

type mobileTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	DeviceID     string `json:"device_id"`
	DeviceStatus string `json:"device_status"`
}

type meDevice struct {
	EmployeeID   string  `json:"employee_id"`
	DeviceStatus *string `json:"device_status"`
}

func mobileLogin(c *client, email, password, device string) response {
	return c.do(http.MethodPost, "/v1/auth/mobile/login", map[string]any{
		"email": email, "password": password,
		"device": map[string]any{"device_hash": deviceHash(device), "model": "Redmi 13C", "os_version": "Android 14"},
	})
}

// bearer membuat klien aplikasi: tanpa cookie, hanya token dan perangkatnya.
func bearer(t *testing.T, srv *httptest.Server, tokens mobileTokens) *client {
	return newClient(t, srv).withHeaders(map[string]string{"Authorization": "Bearer " + tokens.AccessToken, "X-Device-Id": tokens.DeviceID})
}

func TestMobileLoginBindsDevice(t *testing.T) {
	srv := fresh(t, options{})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	anon := newClient(t, srv)

	first := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp-utama").want(t, http.StatusOK))
	if first.DeviceStatus != "approved" || first.AccessToken == "" || first.RefreshToken == "" || first.ExpiresIn != int(auth.AccessTTL.Seconds()) {
		t.Fatalf("login pertama: %+v", first)
	}
	if len(anon.cookies) != 0 {
		t.Errorf("login mobile tidak boleh menyetel cookie: %v", anon.cookies)
	}
	hp := bearer(t, srv, first)
	me := data[meDevice](t, hp.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	if me.EmployeeID != acc.employeeID || me.DeviceStatus == nil || *me.DeviceStatus != "approved" {
		t.Errorf("me = %+v", me)
	}
	hp.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "clock_in"}).want(t, http.StatusCreated)

	// HP yang sama dikenali, bukan didaftarkan ulang.
	again := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp-utama").want(t, http.StatusOK))
	if again.DeviceID != first.DeviceID || again.DeviceStatus != "approved" {
		t.Errorf("login ulang: %+v", again)
	}

	// HP kedua menunggu persetujuan (D32): boleh masuk, tetapi belum boleh presensi.
	second := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp-baru").want(t, http.StatusOK))
	if second.DeviceStatus != "pending" || second.DeviceID == first.DeviceID {
		t.Errorf("HP kedua: %+v", second)
	}
	pending := bearer(t, srv, second)
	if got := data[meDevice](t, pending.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)); got.DeviceStatus == nil || *got.DeviceStatus != "pending" {
		t.Errorf("me HP kedua: %+v", got)
	}
	// Sesi web tidak terikat perangkat.
	if got := data[meDevice](t, login(t, srv, acc.email, seedPassword).do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)); got.DeviceStatus != nil {
		t.Errorf("me web: %+v", got)
	}
	pending.do(http.MethodPost, "/v1/attendance/challenge", map[string]any{"purpose": "clock_in"}).
		wantError(t, http.StatusForbidden, "DEVICE_PENDING_APPROVAL")

	var sessions int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id = $1 AND client = 'mobile' AND device_id IS NOT NULL`, acc.userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 3 {
		t.Errorf("sesi mobile = %d", sessions)
	}
	if countAudit(t, "device.registered") != 2 {
		t.Error("dua perangkat baru harus tercatat")
	}

	// Perangkat dicabut: sesinya mati, login dan refresh dengan HP itu ditolak.
	sdm := loginSDM(t, srv)
	sdm.do(http.MethodPost, "/v1/admin/devices/"+first.DeviceID+"/revoke", nil).want(t, http.StatusOK)
	hp.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	mobileLogin(anon, acc.email, seedPassword, "hp-utama").wantError(t, http.StatusForbidden, "DEVICE_REVOKED")
	anon.do(http.MethodPost, "/v1/auth/mobile/refresh", map[string]any{"refresh_token": again.RefreshToken}).
		wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
}

func TestMobileLoginRejectsBadInput(t *testing.T) {
	srv := fresh(t, options{})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	anon := newClient(t, srv)

	mobileLogin(anon, acc.email, wrongPassword, "hp").wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	f := anon.do(http.MethodPost, "/v1/auth/mobile/login", map[string]any{
		"email": acc.email, "password": seedPassword, "device": map[string]any{"device_hash": "bukan-hash"},
	}).want(t, http.StatusBadRequest).fields()
	if f["device.device_hash"] == nil {
		t.Errorf("hash salah: %v", f)
	}
	// Login gagal tidak meninggalkan perangkat atau sesi.
	var devices, sessions int
	if err := ownerPool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM devices WHERE employee_id = $1), (SELECT count(*) FROM sessions WHERE user_id = $2)`, acc.employeeID, acc.userID).Scan(&devices, &sessions); err != nil {
		t.Fatal(err)
	}
	if devices != 0 || sessions != 0 {
		t.Errorf("perangkat %d, sesi %d", devices, sessions)
	}
}

// Rotasi refresh mobile mengikuti F6, dan token web dan mobile tidak bisa saling dipakai.
func TestMobileRefreshRotation(t *testing.T) {
	srv := fresh(t, options{})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	anon := newClient(t, srv)
	first := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp").want(t, http.StatusOK))

	refresh := func(token string) response {
		return anon.do(http.MethodPost, "/v1/auth/mobile/refresh", map[string]any{"refresh_token": token})
	}
	refresh("").wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	next := data[mobileTokens](t, refresh(first.RefreshToken).want(t, http.StatusOK))
	if next.RefreshToken == first.RefreshToken || next.DeviceID != first.DeviceID || next.DeviceStatus != "approved" {
		t.Fatalf("refresh: %+v", next)
	}
	bearer(t, srv, next).do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	// Sesi web tidak bisa diperpanjang lewat route mobile.
	web := login(t, srv, acc.email, seedPassword)
	refresh(web.cookies[auth.CookieRefresh]).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	web.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	// Token lama dipakai ulang: sesi dicabut.
	refresh(first.RefreshToken).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	bearer(t, srv, next).do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	refresh(next.RefreshToken).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	if countAudit(t, "auth.refresh_reuse_detected") != 1 {
		t.Error("pemakaian ulang harus tercatat")
	}

	// Sesi mobile berakhir 30 hari setelah login dan tidak diperpanjang refresh.
	second := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp").want(t, http.StatusOK))
	var idle, absolute time.Time
	if err := ownerPool.QueryRow(context.Background(), `SELECT idle_expires_at, absolute_expires_at FROM sessions WHERE refresh_hash = $1`, auth.HashToken(second.RefreshToken)).Scan(&idle, &absolute); err != nil {
		t.Fatal(err)
	}
	if d := time.Until(absolute); d < 29*24*time.Hour || d > 31*24*time.Hour || !idle.Equal(absolute) {
		t.Errorf("masa sesi: idle %v, absolute %v", idle, absolute)
	}
	exec(t, `UPDATE sessions SET absolute_expires_at = now() - interval '1 second' WHERE refresh_hash = $1`, auth.HashToken(second.RefreshToken))
	refresh(second.RefreshToken).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	// Logout lewat bearer mencabut sesi itu saja.
	third := data[mobileTokens](t, mobileLogin(anon, acc.email, seedPassword, "hp").want(t, http.StatusOK))
	hp := bearer(t, srv, third)
	hp.do(http.MethodPost, "/v1/auth/logout", nil).want(t, http.StatusNoContent)
	hp.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	web.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)
}

// Akun ber-MFA tidak bisa melewati kodenya lewat aplikasi (D40).
func TestMobileLoginRequiresMFACode(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	setup := data[struct {
		Secret string `json:"secret"`
	}](t, sdm.do(http.MethodPost, "/v1/auth/mfa/setup", nil).want(t, http.StatusOK))
	sdm.do(http.MethodPost, "/v1/auth/mfa/enable", map[string]any{"code": totpCode(t, setup.Secret, time.Now())}).want(t, http.StatusOK)
	exec(t, `UPDATE user_mfa SET last_used_step = 0`)

	anon := newClient(t, srv)
	body := func(code string) map[string]any {
		return map[string]any{"email": sdmEmail, "password": seedPassword, "mfa_code": code, "device": map[string]any{"device_hash": deviceHash("hp-sdm")}}
	}
	anon.do(http.MethodPost, "/v1/auth/mobile/login", body("")).wantError(t, http.StatusUnauthorized, "AUTH_MFA_REQUIRED")
	anon.do(http.MethodPost, "/v1/auth/mobile/login", body("000000")).wantError(t, http.StatusUnauthorized, "AUTH_MFA_INVALID")
	var devices int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM devices`).Scan(&devices); err != nil {
		t.Fatal(err)
	}
	if devices != 0 {
		t.Errorf("perangkat terdaftar sebelum kode benar: %d", devices)
	}
	data[mobileTokens](t, anon.do(http.MethodPost, "/v1/auth/mobile/login", body(totpCode(t, setup.Secret, time.Now()))).want(t, http.StatusOK))
}
