package integration

import (
	"context"
	"net/http"
	"testing"
)

func roleSource(t *testing.T) string {
	t.Helper()
	var source string
	err := ownerPool.QueryRow(context.Background(),
		`SELECT detail->>'source' FROM audit_logs WHERE action = 'user.role_changed' ORDER BY created_at DESC LIMIT 1`).Scan(&source)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// D60: super admin menjadikan Pejuang SDM (dan sebaliknya) serta mengatur A2F wajib per akun SDM.
func TestAccountRoleAndMFAPolicy(t *testing.T) {
	srv := fresh(t, options{})
	admin, adminAcc := superadminClient(t, srv, "OPS-GDG")
	target := activeAccount(t, "pejuang", "OPS-LAP")
	targetClient := login(t, srv, target.email, seedPassword)
	rolePath := "/v1/admin/users/" + target.userID + "/role"
	policyPath := "/v1/admin/users/" + target.userID + "/mfa-policy"

	// SDM tidak punya account:manage.
	loginSDM(t, srv).do(http.MethodPut, rolePath, map[string]any{"role": "sdm"}).want(t, http.StatusForbidden)

	admin.do(http.MethodPut, rolePath, map[string]any{"role": "kadiv"}).want(t, http.StatusBadRequest)
	admin.do(http.MethodPut, "/v1/admin/users/"+adminAcc.userID+"/role", map[string]any{"role": "pejuang"}).
		wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	// A2F hanya bisa diatur untuk akun SDM.
	admin.do(http.MethodPut, policyPath, map[string]any{"required": false}).wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")

	// Jadi SDM: sesi lama dicabut dan peran baru berlaku setelah masuk lagi.
	admin.do(http.MethodPut, rolePath, map[string]any{"role": "sdm"}).want(t, http.StatusNoContent)
	targetClient.do(http.MethodGet, "/v1/me", nil).wantError(t, http.StatusUnauthorized, "AUTH_TOKEN_INVALID")
	again := login(t, srv, target.email, seedPassword)
	me := data[map[string]any](t, again.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	if me["role"] != "sdm" || me["mfa_required"] != true {
		t.Fatalf("setelah jadi SDM: role=%v mfa_required=%v", me["role"], me["mfa_required"])
	}
	if roleSource(t) != "panel" {
		t.Errorf("sumber audit ubah peran: %s", roleSource(t))
	}

	// A2F dijadikan opsional untuk akun SDM ini.
	admin.do(http.MethodPut, policyPath, map[string]any{}).want(t, http.StatusBadRequest)
	admin.do(http.MethodPut, policyPath, map[string]any{"required": false}).want(t, http.StatusNoContent)
	if me := data[map[string]any](t, again.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK)); me["mfa_required"] != false {
		t.Fatalf("mfa_required setelah opsional: %v", me["mfa_required"])
	}
	if countAudit(t, "user.mfa_policy_changed") != 1 {
		t.Errorf("audit kebijakan A2F: %d", countAudit(t, "user.mfa_policy_changed"))
	}

	// Super admin lain dan kepala unit tidak bisa diubah dari panel.
	other := activeAccount(t, "pejuang", "OPS-GDG")
	exec(t, `UPDATE users SET role = 'superadmin' WHERE id = $1`, other.userID)
	admin.do(http.MethodPut, "/v1/admin/users/"+other.userID+"/role", map[string]any{"role": "pejuang"}).
		wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")
	admin.do(http.MethodPut, "/v1/admin/users/"+other.userID+"/mfa-policy", map[string]any{"required": false}).
		wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")
	head := activeAccount(t, "pejuang", "OPS-LAP")
	exec(t, `UPDATE users SET role = 'kadiv' WHERE id = $1`, head.userID)
	admin.do(http.MethodPut, "/v1/admin/users/"+head.userID+"/role", map[string]any{"role": "sdm"}).
		wantError(t, http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED")

	// Kembali ke Pejuang.
	admin.do(http.MethodPut, rolePath, map[string]any{"role": "pejuang"}).want(t, http.StatusNoContent)
	if countAudit(t, "user.role_changed") != 2 {
		t.Errorf("audit ubah peran: %d", countAudit(t, "user.role_changed"))
	}
}

// A2F opsional membebaskan SDM dari kewajiban mendaftar; super admin tetap wajib.
func TestMFAPolicyEnforcement(t *testing.T) {
	srv := fresh(t, options{requireMFA: true})
	sdm := loginSDM(t, srv)
	sdm.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).wantError(t, http.StatusForbidden, "MFA_SETUP_REQUIRED")
	exec(t, `UPDATE users SET mfa_required = false WHERE email = $1`, sdmEmail)
	sdm.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).want(t, http.StatusOK)

	admin, acc := superadminClient(t, srv, "OPS-GDG")
	exec(t, `UPDATE users SET mfa_required = false WHERE id = $1`, acc.userID)
	admin.do(http.MethodGet, "/v1/admin/dashboard/summary", nil).wantError(t, http.StatusForbidden, "MFA_SETUP_REQUIRED")
}
