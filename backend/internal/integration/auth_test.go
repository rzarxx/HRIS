package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/auth"
)

const (
	wrongPassword = "bukan-kata-sandi-ini"
	newPassword   = "kuda-terbang-biru-7" // gitleaks:allow, nilai rekaan khusus tes
)

func TestLoginLogout(t *testing.T) {
	srv := fresh(t, options{})

	anon := newClient(t, srv)
	anon.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	anon.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": sdmEmail, "password": wrongPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	// Email yang tidak ada dijawab sama persis.
	anon.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": "tidak.ada@pejuang.id", "password": wrongPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")

	c := loginSDM(t, srv)
	for _, name := range []string{auth.CookieAccess, auth.CookieRefresh, auth.CookieCSRF} {
		if c.cookies[name] == "" {
			t.Errorf("cookie %s tidak disetel", name)
		}
	}
	me := data[struct {
		Role        string   `json:"role"`
		Email       string   `json:"email"`
		Permissions []string `json:"permissions"`
		MFAEnabled  bool     `json:"mfa_enabled"`
	}](t, c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	if me.Role != "sdm" || me.Email != sdmEmail || len(me.Permissions) == 0 || me.MFAEnabled {
		t.Errorf("me: %+v", me)
	}

	stale := c.clone()
	c.do(http.MethodPost, "/v1/auth/logout", nil).want(t, http.StatusNoContent)
	if len(c.cookies) != 0 {
		t.Errorf("cookie tersisa setelah keluar: %v", c.cookies)
	}
	// Token lama langsung mati karena sesinya dicabut.
	stale.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	if countAudit(t, "auth.login_success") != 1 || countAudit(t, "auth.logout") != 1 {
		t.Error("login dan logout harus tercatat di audit")
	}
	if countAudit(t, "auth.login_failed") != 2 {
		t.Errorf("login_failed = %d, ingin 2", countAudit(t, "auth.login_failed"))
	}
}

func TestCSRFIsEnforcedAgainstRealSession(t *testing.T) {
	srv := fresh(t, options{})
	c := loginSDM(t, srv)
	real := c.cookies[auth.CookieCSRF]
	delete(c.cookies, auth.CookieCSRF)
	c.do(http.MethodPost, "/v1/auth/logout", nil).wantError(t, http.StatusForbidden, "CSRF_INVALID")
	c.cookies[auth.CookieCSRF] = real
	c.do(http.MethodPost, "/v1/auth/logout", nil).want(t, http.StatusNoContent)
}

func TestLockout(t *testing.T) {
	srv := fresh(t, options{})
	c := newClient(t, srv)
	for i := 0; i < 5; i++ {
		c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": sdmEmail, "password": wrongPassword}).
			wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	}
	// Sandi benar pun ditolak selama terkunci.
	locked := c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": sdmEmail, "password": seedPassword}).
		wantError(t, http.StatusLocked, "AUTH_ACCOUNT_LOCKED")
	details, _ := locked.errorField("details").(map[string]any)
	if seconds, _ := details["retry_after_seconds"].(float64); seconds <= 0 || seconds > 15*60+1 {
		t.Errorf("retry_after_seconds = %v", details["retry_after_seconds"])
	}
	if countAudit(t, "user.locked") != 1 {
		t.Error("penguncian harus tercatat sekali")
	}

	// Setelah masa kunci lewat, login normal dan hitungan gagal direset.
	exec(t, `UPDATE users SET locked_until = now() - interval '1 second' WHERE email = $1`, sdmEmail)
	loginSDM(t, srv)
	var failures int
	if err := ownerPool.QueryRow(context.Background(), `SELECT failed_login_count FROM users WHERE email = $1`, sdmEmail).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 0 {
		t.Errorf("failed_login_count = %d", failures)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv := fresh(t, options{limiter: auth.NewMemoryLimiter()})
	c := newClient(t, srv)
	// Login berhasil tidak menghabiskan jatah.
	for i := 0; i < 6; i++ {
		loginSDM(t, srv)
	}
	const other = "tidak.ada@pejuang.id"
	for i := 0; i < 5; i++ {
		c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": other, "password": wrongPassword}).
			want(t, http.StatusUnauthorized)
	}
	limited := c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": other, "password": wrongPassword}).
		wantError(t, http.StatusTooManyRequests, "RATE_LIMITED")
	if limited.header.Get("Retry-After") == "" {
		t.Error("Retry-After harus ada")
	}
	// Akun lain dari IP yang sama tidak ikut tertahan.
	loginSDM(t, srv)
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	srv := fresh(t, options{})
	c := loginSDM(t, srv)
	old := c.clone()

	c.do(http.MethodPost, "/v1/auth/web/refresh", nil).want(t, http.StatusNoContent)
	if c.cookies[auth.CookieRefresh] == old.cookies[auth.CookieRefresh] {
		t.Fatal("refresh token harus berganti")
	}
	c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	// Refresh token lama dipakai ulang: dianggap pencurian, seluruh sesi dicabut.
	old.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	c.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	c.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	if countAudit(t, "auth.refresh_reuse_detected") != 1 {
		t.Error("deteksi reuse harus tercatat")
	}
}

func TestWebRefreshRejectsMobileSession(t *testing.T) {
	srv := fresh(t, options{})
	hp, acc := pejuangClient(t, srv, "OPS-GDG")
	d := data[deviceRow](t, register(hp, "refresh-mobile").want(t, http.StatusCreated))
	const refresh = "token-mobile-khusus-tes"
	const previous = "token-mobile-lama-khusus-tes"
	var sessionID string
	err := ownerPool.QueryRow(context.Background(), `
		INSERT INTO sessions (user_id, client, device_id, refresh_hash, prev_refresh_hash, idle_expires_at, absolute_expires_at)
		VALUES ($1, 'mobile', $2, $3, $4, now() + interval '1 hour', now() + interval '1 day') RETURNING id::text`,
		acc.userID, d.ID, auth.HashToken(refresh), auth.HashToken(previous)).Scan(&sessionID)
	if err != nil {
		t.Fatal(err)
	}
	old := hp.clone()
	hp.cookies[auth.CookieRefresh] = refresh
	hp.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	old.cookies[auth.CookieRefresh] = previous
	old.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	var hash string
	var revoked bool
	if err := ownerPool.QueryRow(context.Background(), `SELECT refresh_hash, revoked_at IS NOT NULL FROM sessions WHERE id = $1`, sessionID).Scan(&hash, &revoked); err != nil {
		t.Fatal(err)
	}
	if hash != auth.HashToken(refresh) || revoked {
		t.Error("refresh web tidak boleh mengubah sesi mobile")
	}
}

func TestRefreshRejections(t *testing.T) {
	srv := fresh(t, options{})

	c := loginSDM(t, srv)
	exec(t, `UPDATE sessions SET idle_expires_at = now() - interval '1 second'`)
	c.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	c = loginSDM(t, srv)
	exec(t, `UPDATE sessions SET absolute_expires_at = now() - interval '1 second'`)
	c.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	c = loginSDM(t, srv)
	delete(c.cookies, auth.CookieCSRF)
	c.do(http.MethodPost, "/v1/auth/web/refresh", nil).wantError(t, http.StatusForbidden, "CSRF_INVALID")

	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/refresh", nil).want(t, http.StatusForbidden)
}

type createdEmployee struct {
	ID              string `json:"id"`
	Nip             string `json:"nip"`
	FullName        string `json:"full_name"`
	InitialPassword string `json:"initial_password"`
	ExpiresAt       string `json:"expires_at"`
	Account         struct {
		UserID   string `json:"user_id"`
		Role     string `json:"role"`
		IsActive bool   `json:"is_active"`
	} `json:"account"`
	BaseSalary string `json:"base_salary"`
}

func branchID(t *testing.T, code string) string {
	t.Helper()
	var id string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM branches WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func subdivisionID(t *testing.T, code string) string {
	t.Helper()
	var id string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM subdivisions WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func createEmployee(t *testing.T, sdm *client, email string) createdEmployee {
	t.Helper()
	return data[createdEmployee](t, sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name":      "Rudi Hartono",
		"email":          email,
		"subdivision_id": subdivisionID(t, "OPS-LAP"),
		"base_salary":    "4250000.00",
	}).want(t, http.StatusCreated))
}

// changePassword mengganti kata sandi awal, syarat sebelum fitur lain bisa dipakai.
func changePassword(t *testing.T, c *client, current, next string) {
	t.Helper()
	c.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": current, "new_password": next}).want(t, http.StatusNoContent)
}

// D53: akun baru langsung aktif dengan kata sandi awal @NamaDepan+tahun yang wajib diganti.
func TestInitialPasswordFlow(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	const email = "rudi.hartono.baru@pejuang.id"
	created := createEmployee(t, sdm, email)
	want := auth.InitialPassword("Rudi Hartono", time.Now())
	if created.InitialPassword != want || created.Nip == "" {
		t.Fatalf("respons pembuatan: %+v, mau kata sandi %s", created, want)
	}
	if !created.Account.IsActive || created.Account.Role != "pejuang" {
		t.Errorf("akun baru harus pejuang dan aktif: %+v", created.Account)
	}
	var stored string
	if err := ownerPool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id = $1`, created.Account.UserID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, created.InitialPassword) {
		t.Error("kata sandi awal tersimpan apa adanya")
	}

	// Masuk dengan NIPY; fitur lain tertutup sampai kata sandi diganti.
	c := newClient(t, srv)
	c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": created.Nip, "password": created.InitialPassword}).want(t, http.StatusOK)
	me := data[struct {
		Role               string `json:"role"`
		MustChangePassword bool   `json:"must_change_password"`
	}](t, c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	if me.Role != "pejuang" || !me.MustChangePassword {
		t.Errorf("me = %+v", me)
	}
	c.do(http.MethodGet, "/v1/announcements/active", nil).wantError(t, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
	if f := c.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": created.InitialPassword, "new_password": "pendek"}).
		want(t, http.StatusBadRequest).fields(); f["new_password"] == nil {
		t.Errorf("sandi pendek: %v", f)
	}
	changePassword(t, c, created.InitialPassword, newPassword)

	// Kata sandi awal tidak berlaku lagi; sandi baru berlaku lewat email.
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": created.Nip, "password": created.InitialPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	login(t, srv, email, newPassword).do(http.MethodGet, "/v1/announcements/active", nil).want(t, http.StatusOK)
	if countAudit(t, "employee.created") != 1 || countAudit(t, "user.created") != 1 || countAudit(t, "auth.password_changed") != 1 {
		t.Error("pembuatan akun dan ganti kata sandi harus tercatat")
	}

	// Akun lama yang belum pernah aktivasi (sebelum D53) aktif setelah SDM mereset kata sandinya.
	if _, err := ownerPool.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, created.Account.UserID); err != nil {
		t.Fatal(err)
	}
	reset := data[auth.InitialCredential](t, sdm.do(http.MethodPost, "/v1/admin/users/"+created.Account.UserID+"/reset-password", nil).want(t, http.StatusOK))
	if reset.InitialPassword != want {
		t.Errorf("reset = %q, mau %q", reset.InitialPassword, want)
	}
	login(t, srv, created.Nip, reset.InitialPassword).do(http.MethodGet, "/v1/announcements/active", nil).wantError(t, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
}

// D53: kata sandi awal hanya berlaku 72 jam; lewat dari itu SDM harus mereset ulang.
func TestInitialPasswordExpires(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	created := createEmployee(t, sdm, "kedaluwarsa@pejuang.id")
	expires, err := time.Parse(time.RFC3339, created.ExpiresAt)
	if err != nil {
		t.Fatalf("expires_at = %q: %v", created.ExpiresAt, err)
	}
	if d := time.Until(expires); d < auth.InitialPasswordTTL-time.Minute || d > auth.InitialPasswordTTL {
		t.Errorf("berlaku %v, mau sekitar %v", d, auth.InitialPasswordTTL)
	}
	exec(t, `UPDATE users SET initial_password_expires_at = now() - interval '1 minute' WHERE id = $1`, created.Account.UserID)

	// Kata sandi salah tetap dijawab umum; kata sandi benar yang kedaluwarsa dijawab khusus.
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": created.Nip, "password": wrongPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": created.Nip, "password": created.InitialPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INITIAL_PASSWORD_EXPIRED")

	// Reset menerbitkan kata sandi awal baru dengan batas baru.
	reset := data[auth.InitialCredential](t, sdm.do(http.MethodPost, "/v1/admin/users/"+created.Account.UserID+"/reset-password", nil).want(t, http.StatusOK))
	if reset.ExpiresAt == "" {
		t.Fatal("reset tanpa expires_at")
	}
	c := login(t, srv, created.Nip, reset.InitialPassword)
	changePassword(t, c, reset.InitialPassword, newPassword)

	// Kata sandi pilihan sendiri tidak ikut kedaluwarsa.
	var stillSet bool
	if err := ownerPool.QueryRow(context.Background(), `SELECT initial_password_expires_at IS NOT NULL FROM users WHERE id = $1`, created.Account.UserID).Scan(&stillSet); err != nil {
		t.Fatal(err)
	}
	if stillSet {
		t.Error("batas kata sandi awal harus hilang setelah diganti")
	}
}

// D53: login bisa memakai NIPY, email, atau nomor WA dalam bentuk apa pun.
func TestLoginIdentifiers(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	created := data[createdEmployee](t, sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name":      "Siti Aminah",
		"nip":            "2026.01.777",
		"phone":          "0812-3456-7777",
		"subdivision_id": subdivisionID(t, "OPS-LAP"),
		"base_salary":    "3000000.00",
	}).want(t, http.StatusCreated))
	if created.InitialPassword == "" {
		t.Fatal("kata sandi awal kosong")
	}
	for _, id := range []string{"2026.01.777", "2026.01.777 ", "081234567777", "+62 812 3456 7777", "6281234567777"} {
		newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": id, "password": created.InitialPassword}).want(t, http.StatusOK)
	}
	// Field email lama tetap diterima (aplikasi versi lama).
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": "2026.01.777", "password": created.InitialPassword}).want(t, http.StatusOK)
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": "089999999999", "password": created.InitialPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"identifier": "", "password": "x"}).want(t, http.StatusBadRequest)

	// Nomor WA unik dan disimpan dalam bentuk baku.
	if f := sdm.do(http.MethodPost, "/v1/admin/employees", map[string]any{
		"full_name": "Pemakai Nomor Sama", "phone": "+6281234567777", "subdivision_id": subdivisionID(t, "OPS-LAP"), "base_salary": "1.00",
	}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE").fields(); f["phone"] == nil {
		t.Errorf("duplikat WA: %v", f)
	}
}

func TestPasswordChangeRevokesOtherSessions(t *testing.T) {
	srv := fresh(t, options{})
	current := loginSDM(t, srv)
	other := loginSDM(t, srv)

	if f := current.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": wrongPassword, "new_password": newPassword}).
		want(t, http.StatusBadRequest).fields(); f["current_password"] == nil {
		t.Errorf("sandi lama salah: %v", f)
	}
	current.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": seedPassword, "new_password": seedPassword}).
		want(t, http.StatusBadRequest)
	current.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": seedPassword, "new_password": newPassword}).
		want(t, http.StatusNoContent)

	current.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)
	other.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": sdmEmail, "password": seedPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
	login(t, srv, sdmEmail, newPassword)
}

func TestMustChangePasswordGate(t *testing.T) {
	srv := fresh(t, options{})
	c := loginSDM(t, srv)
	exec(t, `UPDATE users SET must_change_password = true WHERE email = $1`, sdmEmail)
	c.do(http.MethodGet, "/v1/admin/divisions", nil).wantError(t, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
	c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)
	c.do(http.MethodPost, "/v1/auth/password/change", map[string]any{"current_password": seedPassword, "new_password": newPassword}).
		want(t, http.StatusNoContent)
	c.do(http.MethodGet, "/v1/admin/divisions", nil).want(t, http.StatusOK)
}

func TestSessions(t *testing.T) {
	srv := fresh(t, options{})
	current := loginSDM(t, srv)
	other := loginSDM(t, srv)

	type session struct {
		ID        string `json:"id"`
		Client    string `json:"client"`
		IsCurrent bool   `json:"is_current"`
	}
	list := data[[]session](t, current.do(http.MethodGet, "/v1/auth/sessions", nil).want(t, http.StatusOK))
	if len(list) != 2 {
		t.Fatalf("jumlah sesi = %d", len(list))
	}
	var otherID string
	for _, s := range list {
		if !s.IsCurrent {
			otherID = s.ID
		}
	}
	if otherID == "" {
		t.Fatal("sesi lain tidak ditemukan")
	}

	// Pejuang lain tidak bisa mencabut sesi milik SDM.
	stranger := activeAccount(t, "pejuang", "OPS-GDG")
	login(t, srv, stranger.email, seedPassword).do(http.MethodDelete, "/v1/auth/sessions/"+otherID, nil).
		wantError(t, http.StatusNotFound, "NOT_FOUND")
	other.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	current.do(http.MethodDelete, "/v1/auth/sessions/"+otherID, nil).want(t, http.StatusNoContent)
	other.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
}

// totpCode menghitung kode independen dari kode aplikasi, sesuai RFC 6238.
func totpCode(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1_000_000)
}

func loginMFAToken(t *testing.T, srv interface{ Close() }, c *client) string {
	t.Helper()
	res := c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": sdmEmail, "password": seedPassword}).
		wantError(t, http.StatusUnauthorized, "AUTH_MFA_REQUIRED")
	details, _ := res.errorField("details").(map[string]any)
	token, _ := details["mfa_token"].(string)
	if token == "" {
		t.Fatalf("mfa_token kosong: %s", res.body)
	}
	return token
}

func TestMFAFlow(t *testing.T) {
	srv := fresh(t, options{requireMFA: true})
	c := loginSDM(t, srv)

	// SDM tanpa MFA terkunci dari fitur lain, tetapi bisa mendaftar.
	c.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).wantError(t, http.StatusForbidden, "MFA_SETUP_REQUIRED")
	c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)

	setup := data[struct {
		Secret     string `json:"secret"`
		OtpauthURL string `json:"otpauth_url"`
	}](t, c.do(http.MethodPost, "/v1/auth/mfa/setup", nil).want(t, http.StatusOK))
	if setup.Secret == "" || !strings.HasPrefix(setup.OtpauthURL, "otpauth://totp/") {
		t.Fatalf("setup: %+v", setup)
	}

	// Secret tersimpan terenkripsi.
	var enc []byte
	if err := ownerPool.QueryRow(context.Background(), `SELECT secret_enc FROM user_mfa`).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), setup.Secret) {
		t.Error("secret TOTP tersimpan apa adanya")
	}

	c.do(http.MethodPost, "/v1/auth/mfa/enable", map[string]any{"code": "000000"}).wantError(t, http.StatusUnauthorized, "AUTH_MFA_INVALID")
	enabled := data[struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}](t, c.do(http.MethodPost, "/v1/auth/mfa/enable", map[string]any{"code": totpCode(t, setup.Secret, time.Now())}).want(t, http.StatusOK))
	if len(enabled.RecoveryCodes) != 8 {
		t.Fatalf("kode pemulihan = %d", len(enabled.RecoveryCodes))
	}
	c.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).want(t, http.StatusOK)
	c.do(http.MethodPost, "/v1/auth/mfa/setup", nil).wantError(t, http.StatusConflict, "MFA_ALREADY_ENABLED")
	if countAudit(t, "auth.mfa_enabled") != 1 {
		t.Error("aktivasi MFA harus tercatat")
	}

	// Login kini dua langkah; langkah sandi belum memberi sesi.
	second := newClient(t, srv)
	token := loginMFAToken(t, srv, second)
	if second.cookies[auth.CookieAccess] != "" {
		t.Error("cookie sesi tidak boleh terbit sebelum kode benar")
	}
	verify := func(c *client, token, code string) response {
		return c.do(http.MethodPost, "/v1/auth/web/mfa/verify", map[string]any{"mfa_token": token, "code": code})
	}
	verify(second, token, "000000").wantError(t, http.StatusUnauthorized, "AUTH_MFA_INVALID")
	if countAudit(t, "auth.mfa_failed") != 1 {
		t.Error("kode salah harus tercatat")
	}
	verify(second, "token-palsu", "000000").wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	// Kode dari langkah aktivasi sudah terpakai; izinkan satu langkah waktu berikutnya.
	code := totpCode(t, setup.Secret, time.Now())
	exec(t, `UPDATE user_mfa SET last_used_step = 0`)
	verify(second, token, code).want(t, http.StatusOK)
	second.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).want(t, http.StatusOK)

	// Kode yang sama tidak bisa dipakai ulang.
	third := newClient(t, srv)
	verify(third, loginMFAToken(t, srv, third), code).wantError(t, http.StatusUnauthorized, "AUTH_MFA_INVALID")

	// Kode pemulihan berlaku sekali.
	token = loginMFAToken(t, srv, third)
	verify(third, token, enabled.RecoveryCodes[0]).want(t, http.StatusOK)
	fourth := newClient(t, srv)
	verify(fourth, loginMFAToken(t, srv, fourth), enabled.RecoveryCodes[0]).wantError(t, http.StatusUnauthorized, "AUTH_MFA_INVALID")
	verify(fourth, loginMFAToken(t, srv, fourth), enabled.RecoveryCodes[1]).want(t, http.StatusOK)

	// Token MFA tidak bisa dipakai sebagai access token.
	forged := newClient(t, srv)
	forged.cookies[auth.CookieAccess] = loginMFAToken(t, srv, forged)
	forged.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")

	// Akun yang dikunci saat menunggu kode tidak bisa menyelesaikan login.
	pending := newClient(t, srv)
	token = loginMFAToken(t, srv, pending)
	exec(t, `UPDATE users SET locked_until = now() + interval '1 hour' WHERE email = $1`, sdmEmail)
	verify(pending, token, enabled.RecoveryCodes[2]).wantError(t, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
}
