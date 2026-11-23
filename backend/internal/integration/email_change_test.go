package integration

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/mail"
)

var confirmLink = regexp.MustCompile(`https://admin\.test/konfirmasi-email/#token=([A-Za-z0-9_-]+)`)

const smtpRow = `INSERT INTO app_settings (key, value) VALUES ('smtp', '{"host":"smtp.example.test","port":587,"from_email":"hris@albahjah.or.id"}')`

// mailTo mencari email terakhir untuk satu alamat dari n email pertama yang terkirim.
func mailTo(t *testing.T, box *mailbox, n int, to string) mail.Message {
	t.Helper()
	msgs := box.wait(t, n)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].To == to {
			return msgs[i]
		}
	}
	t.Fatalf("tidak ada email ke %s di %d email", to, n)
	return mail.Message{}
}

func confirmToken(t *testing.T, msg mail.Message) string {
	t.Helper()
	m := confirmLink.FindStringSubmatch(msg.Body)
	if m == nil {
		t.Fatalf("email tanpa tautan konfirmasi: %q", msg.Body)
	}
	return m[1]
}

func pendingEmail(t *testing.T, c *client) any {
	t.Helper()
	return data[map[string]any](t, c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))["pending_email"]
}

func loginStatus(t *testing.T, srv *httptest.Server, email, password string) int {
	t.Helper()
	return newClient(t, srv).do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": email, "password": password}).status
}

// D59: ganti email sendiri menunggu konfirmasi dari email baru; email lama diberi tahu.
func TestEmailChangeNeedsConfirmation(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box})
	exec(t, smtpRow)
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	other := activeAccount(t, "pejuang", "OPS-LAP")
	c := login(t, srv, acc.email, seedPassword)
	const newEmail = "baru.sekali@pejuang.test"

	// Email yang dipakai Pejuang lain langsung ditolak tanpa email apa pun.
	c.do(http.MethodPatch, "/v1/me", map[string]any{"email": other.email, "current_password": seedPassword}).
		wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")

	me := data[map[string]any](t, c.do(http.MethodPatch, "/v1/me", map[string]any{"email": newEmail, "current_password": seedPassword}).want(t, http.StatusOK))
	if me["email"] != acc.email || me["pending_email"] != newEmail {
		t.Fatalf("email belum boleh berubah sebelum konfirmasi: %v", me)
	}
	first := confirmToken(t, mailTo(t, box, 2, newEmail))
	if notice := mailTo(t, box, 2, acc.email); !strings.Contains(notice.Body, "ba***@pejuang.test") || strings.Contains(notice.Body, newEmail) {
		t.Errorf("pemberitahuan ke email lama harus menyamarkan alamat baru: %q", notice.Body)
	}
	if got := loginStatus(t, srv, newEmail, seedPassword); got == http.StatusOK {
		t.Fatal("email baru dipakai masuk sebelum dikonfirmasi")
	}

	// Batal membuat tautan tidak berlaku.
	c.do(http.MethodDelete, "/v1/me/email-change", nil).want(t, http.StatusNoContent)
	if pendingEmail(t, c) != nil {
		t.Fatal("permintaan masih tertunda setelah dibatalkan")
	}
	anon := newClient(t, srv)
	anon.do(http.MethodPost, "/v1/auth/email/confirm", map[string]any{"token": first}).wantError(t, http.StatusBadRequest, "EMAIL_CHANGE_INVALID")

	// Ajukan lagi, lalu konfirmasi: email berubah, sesi lama dicabut, email lama diberi tahu.
	c.do(http.MethodPatch, "/v1/me", map[string]any{"email": newEmail, "current_password": seedPassword}).want(t, http.StatusOK)
	second := confirmToken(t, mailTo(t, box, 4, newEmail))
	anon.do(http.MethodPost, "/v1/auth/email/confirm", map[string]any{"token": second}).want(t, http.StatusNoContent)
	anon.do(http.MethodPost, "/v1/auth/email/confirm", map[string]any{"token": second}).wantError(t, http.StatusBadRequest, "EMAIL_CHANGE_INVALID")
	c.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	if got := loginStatus(t, srv, acc.email, seedPassword); got == http.StatusOK {
		t.Error("email lama masih bisa dipakai masuk")
	}
	fresh := login(t, srv, newEmail, seedPassword)
	if me := data[map[string]any](t, fresh.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)); me["email"] != newEmail || me["pending_email"] != nil {
		t.Fatalf("profil setelah konfirmasi: %v", me)
	}
	if changed := mailTo(t, box, 5, acc.email); !strings.Contains(changed.Subject, "diganti") {
		t.Errorf("pemberitahuan email diganti: %+v", changed)
	}
	if countAudit(t, "user.email_change_requested") != 2 || countAudit(t, "user.email_changed") != 1 {
		t.Errorf("audit: diminta=%d diganti=%d", countAudit(t, "user.email_change_requested"), countAudit(t, "user.email_changed"))
	}

	// Tautan kedaluwarsa ditolak, dan permintaan dibatasi 3 per jam.
	fresh.do(http.MethodPatch, "/v1/me", map[string]any{"email": "ketiga@pejuang.test", "current_password": seedPassword}).want(t, http.StatusOK)
	expired := confirmToken(t, mailTo(t, box, 7, "ketiga@pejuang.test"))
	exec(t, `UPDATE email_change_requests SET expires_at = now() - interval '1 minute' WHERE used_at IS NULL`)
	anon.do(http.MethodPost, "/v1/auth/email/confirm", map[string]any{"token": expired}).wantError(t, http.StatusBadRequest, "EMAIL_CHANGE_INVALID")
	fresh.do(http.MethodPatch, "/v1/me", map[string]any{"email": "keempat@pejuang.test", "current_password": seedPassword}).
		wantError(t, http.StatusTooManyRequests, "RATE_LIMITED")
}

// Tanpa SMTP email berubah langsung; perubahan oleh SDM langsung berlaku, mencabut sesi, dan memberi tahu.
func TestEmailChangeWithoutSMTPAndByAdmin(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box})
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	c := login(t, srv, acc.email, seedPassword)

	me := data[map[string]any](t, c.do(http.MethodPatch, "/v1/me", map[string]any{"email": "langsung@pejuang.test", "current_password": seedPassword}).want(t, http.StatusOK))
	if me["email"] != "langsung@pejuang.test" || me["pending_email"] != nil {
		t.Fatalf("tanpa SMTP email berubah langsung: %v", me)
	}
	if box.count() != 0 || countAudit(t, "user.email_changed") != 1 {
		t.Fatalf("tanpa SMTP: email=%d audit=%d", box.count(), countAudit(t, "user.email_changed"))
	}

	exec(t, smtpRow)
	pejuang := login(t, srv, "langsung@pejuang.test", seedPassword)
	sdm := loginSDM(t, srv)
	sdm.do(http.MethodPatch, "/v1/admin/employees/"+acc.employeeID, map[string]any{"email": "dari.sdm@pejuang.test"}).want(t, http.StatusOK)
	pejuang.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	mailTo(t, box, 2, "langsung@pejuang.test")
	mailTo(t, box, 2, "dari.sdm@pejuang.test")
	login(t, srv, "dari.sdm@pejuang.test", seedPassword)
	if countAudit(t, "user.email_changed") != 2 {
		t.Errorf("audit email diganti SDM: %d", countAudit(t, "user.email_changed"))
	}
}
