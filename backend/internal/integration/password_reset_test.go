package integration

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/mail"
)

var resetLink = regexp.MustCompile(`https://admin\.test/reset-kata-sandi/#token=([A-Za-z0-9_-]+)`)

func resetToken(t *testing.T, msg mail.Message) string {
	t.Helper()
	m := resetLink.FindStringSubmatch(msg.Body)
	if m == nil {
		t.Fatalf("email tanpa tautan reset: %q", msg.Body)
	}
	return m[1]
}

// D30: lupa kata sandi lewat email. Jawaban selalu sama, tautan sekali pakai, dan sesi lama dicabut.
func TestForgotAndResetPassword(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	exec(t, `INSERT INTO app_settings (key, value) VALUES ('smtp', '{"host":"smtp.example.test","port":587,"from_email":"hris@albahjah.or.id"}')`)
	old := login(t, srv, acc.email, seedPassword)

	anon := newClient(t, srv)
	forgot := func(identifier string) response {
		return anon.do(http.MethodPost, "/v1/auth/password/forgot", map[string]any{"identifier": identifier}).want(t, http.StatusAccepted)
	}
	// Akun ada, tidak ada, dan tanpa email dijawab sama persis.
	exists := forgot(acc.email)
	missing := forgot("tidak-ada@pejuang.test")
	noEmail := activeAccount(t, "pejuang", "OPS-LAP")
	exec(t, `UPDATE users SET email = NULL WHERE id = $1`, noEmail.userID)
	var nip string
	if err := ownerPool.QueryRow(context.Background(), `SELECT e.nip FROM employees e JOIN users u ON u.employee_id = e.id WHERE u.id = $1`, noEmail.userID).Scan(&nip); err != nil {
		t.Fatal(err)
	}
	withoutEmail := forgot(nip)
	if string(exists.body) != string(missing.body) || string(exists.body) != string(withoutEmail.body) {
		t.Fatalf("jawaban berbeda:\n%s\n%s\n%s", exists.body, missing.body, withoutEmail.body)
	}
	msgs := box.wait(t, 1)
	if len(msgs) != 1 || msgs[0].To != acc.email {
		t.Fatalf("email terkirim: %+v", msgs)
	}
	first := resetToken(t, msgs[0])

	// Tautan baru membatalkan yang lama.
	forgot(acc.email)
	second := resetToken(t, box.wait(t, 2)[1])
	anon.do(http.MethodPost, "/v1/auth/password/reset", map[string]any{"token": first, "password": "Kebun-Kurma-2026"}).
		wantError(t, http.StatusBadRequest, "RESET_TOKEN_INVALID")

	// Kata sandi lemah tidak menghabiskan tautan.
	weak := anon.do(http.MethodPost, "/v1/auth/password/reset", map[string]any{"token": second, "password": "pendek"}).want(t, http.StatusBadRequest)
	if weak.fields()["password"] == nil {
		t.Fatalf("galat kata sandi lemah: %s", weak.body)
	}
	exec(t, `UPDATE users SET locked_until = now() + interval '10 minutes', failed_login_count = 5 WHERE id = $1`, acc.userID)
	anon.do(http.MethodPost, "/v1/auth/password/reset", map[string]any{"token": second, "password": "Kebun-Kurma-2026"}).want(t, http.StatusNoContent)
	anon.do(http.MethodPost, "/v1/auth/password/reset", map[string]any{"token": second, "password": "Kebun-Kurma-2027"}).
		wantError(t, http.StatusBadRequest, "RESET_TOKEN_INVALID")

	// Sesi lama dicabut, kunci dibuka, dan kata sandi baru berlaku.
	old.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	login(t, srv, acc.email, "Kebun-Kurma-2026")
	if countAudit(t, "auth.password_reset_requested") != 2 || countAudit(t, "auth.password_reset_completed") != 1 {
		t.Errorf("audit reset: diminta=%d selesai=%d", countAudit(t, "auth.password_reset_requested"), countAudit(t, "auth.password_reset_completed"))
	}
	// Pemberitahuan keamanan menyusul ke email terdaftar.
	notice := box.wait(t, 3)[2]
	if notice.To != acc.email || !strings.Contains(notice.Subject, "keamanan") {
		t.Errorf("pemberitahuan keamanan: %+v", notice)
	}

	// Token kedaluwarsa ditolak.
	forgot(acc.email)
	expired := resetToken(t, box.wait(t, 4)[3])
	exec(t, `UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute' WHERE used_at IS NULL`)
	anon.do(http.MethodPost, "/v1/auth/password/reset", map[string]any{"token": expired, "password": "Kebun-Kurma-2028"}).
		wantError(t, http.StatusBadRequest, "RESET_TOKEN_INVALID")
}

// Tanpa SMTP, permintaan tetap 202 tetapi tidak ada email; batas per akun berlaku.
func TestForgotPasswordWithoutSMTPAndRateLimit(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box, limiter: auth.NewMemoryLimiter()})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	anon := newClient(t, srv)
	for i := 0; i < 3; i++ {
		anon.do(http.MethodPost, "/v1/auth/password/forgot", map[string]any{"identifier": acc.email}).want(t, http.StatusAccepted)
	}
	anon.do(http.MethodPost, "/v1/auth/password/forgot", map[string]any{"identifier": acc.email}).wantError(t, http.StatusTooManyRequests, "RATE_LIMITED")
	if box.count() != 0 || countAudit(t, "auth.password_reset_requested") != 0 {
		t.Errorf("tanpa SMTP tidak boleh ada email: %d", box.count())
	}
}

// Pengaturan SMTP hanya tersimpan setelah email uji terkirim; kata sandi tidak pernah kembali.
func TestSMTPSettings(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box})
	admin, acc := superadminClient(t, srv, "OPS-GDG")

	input := map[string]any{
		"host": "smtp.gmail.com", "port": 587, "username": "hris@albahjah.or.id", "password": "rahasia-smtp-123",
		"from_name": "HRIS Al Bahjah", "from_email": "hris@albahjah.or.id",
	}
	box.fail = &mail.SendError{Stage: mail.StageAuth, Err: errors.New("535 salah")}
	failed := admin.do(http.MethodPut, "/v1/admin/settings/smtp", input).wantError(t, http.StatusUnprocessableEntity, "SMTP_TEST_FAILED")
	if !strings.Contains(string(failed.body), `"stage":"auth"`) {
		t.Errorf("tahap gagal: %s", failed.body)
	}
	empty := data[map[string]any](t, admin.do(http.MethodGet, "/v1/admin/settings/smtp", nil).want(t, http.StatusOK))
	if empty["configured"] != false {
		t.Fatalf("pengaturan gagal uji tersimpan: %v", empty)
	}

	box.fail = nil
	saved := admin.do(http.MethodPut, "/v1/admin/settings/smtp", input).want(t, http.StatusOK)
	if strings.Contains(string(saved.body), "rahasia-smtp-123") {
		t.Fatal("kata sandi SMTP ikut di respons")
	}
	view := data[map[string]any](t, saved)
	if view["configured"] != true || view["password_set"] != true || view["host"] != "smtp.gmail.com" {
		t.Fatalf("pengaturan: %v", view)
	}
	if got := box.wait(t, 1)[0]; got.To != acc.email {
		t.Errorf("email uji ke %s, ingin %s", got.To, acc.email)
	}
	var stored, detail string
	if err := ownerPool.QueryRow(context.Background(), `SELECT value FROM app_settings WHERE key = 'smtp'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(context.Background(), `SELECT detail::text FROM audit_logs WHERE action = 'settings.changed' ORDER BY created_at DESC LIMIT 1`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "rahasia-smtp-123") || strings.Contains(detail, "rahasia-smtp-123") {
		t.Fatal("kata sandi SMTP tersimpan atau tercatat tanpa enkripsi")
	}

	// Kata sandi kosong berarti tetap memakai yang lama; port selain 465/587 dan host internal ditolak.
	input["password"] = ""
	admin.do(http.MethodPut, "/v1/admin/settings/smtp", input).want(t, http.StatusOK)
	input["port"] = 25
	if f := admin.do(http.MethodPut, "/v1/admin/settings/smtp", input).want(t, http.StatusBadRequest).fields(); f["port"] == nil {
		t.Errorf("port 25 diterima: %v", f)
	}

	// SDM tidak boleh mengatur SMTP.
	loginSDM(t, srv).do(http.MethodGet, "/v1/admin/settings/smtp", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	admin.do(http.MethodDelete, "/v1/admin/settings/smtp", nil).want(t, http.StatusNoContent)
	admin.do(http.MethodDelete, "/v1/admin/settings/smtp", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
}
