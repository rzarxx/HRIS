package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/auth"
)

func hasMFA(t *testing.T, userID string) bool {
	t.Helper()
	var n int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM user_mfa WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// D71: super admin mereset A2F akun SDM atau super admin lain; akun sendiri lewat perintah server.
func TestResetMFA(t *testing.T) {
	srv := fresh(t, options{})
	admin, adminAcc := superadminClient(t, srv, "OPS-GDG")
	sdm := loginSDM(t, srv)
	var sdmUser string
	if err := ownerPool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE email = $1`, sdmEmail).Scan(&sdmUser); err != nil {
		t.Fatal(err)
	}
	pejuang := activeAccount(t, "pejuang", "OPS-LAP")
	enable := func(userID string) {
		exec(t, `INSERT INTO user_mfa (user_id, secret_enc, enabled_at, recovery_hashes) VALUES ($1, '\x00', now(), '{a}')
			ON CONFLICT (user_id) DO UPDATE SET enabled_at = now()`, userID)
	}
	enable(sdmUser)
	enable(adminAcc.userID)
	path := func(id string) string { return "/v1/admin/users/" + id + "/mfa" }

	// SDM tidak punya account:manage.
	sdm.do(http.MethodDelete, path(adminAcc.userID), nil).want(t, http.StatusForbidden)
	admin.do(http.MethodDelete, path(adminAcc.userID), nil).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	admin.do(http.MethodDelete, path(pejuang.userID), nil).wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")
	admin.do(http.MethodDelete, path("00000000-0000-0000-0000-000000000001"), nil).wantError(t, http.StatusNotFound, "NOT_FOUND")

	admin.do(http.MethodDelete, path(sdmUser), nil).want(t, http.StatusNoContent)
	if hasMFA(t, sdmUser) {
		t.Error("A2F SDM masih ada")
	}
	sdm.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	admin.do(http.MethodDelete, path(sdmUser), nil).wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")
	if n := countAudit(t, "user.mfa_reset"); n != 1 {
		t.Errorf("audit user.mfa_reset %d, ingin 1", n)
	}

	// Perintah server untuk super admin sendiri; akun selain super admin ditolak.
	if err := auth.ResetSuperadminMFA(context.Background(), appPool, sdmEmail); err == nil {
		t.Error("reset-mfa untuk akun SDM harus ditolak")
	}
	if err := auth.ResetSuperadminMFA(context.Background(), appPool, adminAcc.email); err != nil {
		t.Fatal(err)
	}
	if hasMFA(t, adminAcc.userID) {
		t.Error("A2F super admin masih ada")
	}
	admin.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
}
