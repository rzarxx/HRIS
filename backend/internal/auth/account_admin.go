package auth

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Peran SDM dan A2F wajib per akun diatur super admin dari panel (D60, memperbarui D25).

func errAccountChange(message string) error {
	return httpx.NewError(http.StatusConflict, "ACCOUNT_CHANGE_NOT_ALLOWED", message)
}

// accountTarget memuat akun lain yang aktif; akun sendiri dan akun yang tidak ada ditolak.
func (s *Service) accountTarget(ctx context.Context, q *store.Queries, actor Principal, id uuid.UUID) (store.User, error) {
	if id == actor.UserID {
		return store.User{}, errSelfAction("Peran dan A2F akun sendiri diatur super admin lain.")
	}
	user, err := q.GetUserByID(ctx, id)
	if pgconv.IsNoRows(err) {
		return user, httpx.ErrNotFound()
	}
	return user, err
}

// SetAccountRole menjadikan Pejuang sebagai SDM atau mengembalikan SDM menjadi Pejuang. Kepala unit
// mengikuti struktur organisasi dan super admin hanya diatur lewat perintah server, jadi keduanya
// ditolak. Sesi akun dicabut agar peran baru langsung berlaku.
func (s *Service) SetAccountRole(ctx context.Context, actor Principal, id uuid.UUID, role string, ip *netip.Addr) error {
	var to store.UserRole
	switch role {
	case string(store.UserRoleSdm):
		to = store.UserRoleSdm
	case string(store.UserRolePejuang):
		to = store.UserRolePejuang
	default:
		return httpx.Validation(map[string]string{"role": "Pilih sdm atau pejuang."})
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		user, err := s.accountTarget(ctx, q, actor, id)
		if err != nil {
			return err
		}
		if user.Role == to {
			return nil
		}
		switch user.Role {
		case store.UserRolePejuang, store.UserRoleSdm:
		case store.UserRoleSuperadmin:
			return errAccountChange("Peran super admin hanya diubah lewat perintah di server.")
		default:
			return errAccountChange("Akun ini kepala unit. Lepas dulu dari jabatan kepala di Struktur Organisasi.")
		}
		status, err := q.GetEmployeeStatus(ctx, user.EmployeeID)
		if err != nil {
			return err
		}
		if status != "active" {
			return errAccountChange("Pejuang ini sudah nonaktif.")
		}
		return changeRole(ctx, q, user, to, audit.Entry{UserID: audit.Actor(actor.UserID), IP: ip}, "panel")
	})
}

// SetMFAPolicy menjadikan A2F wajib atau opsional untuk satu akun SDM. Super admin selalu wajib.
// Menjadikannya opsional tidak mematikan A2F yang sudah aktif; A2F tetap diminta saat masuk.
func (s *Service) SetMFAPolicy(ctx context.Context, actor Principal, id uuid.UUID, required bool, ip *netip.Addr) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		user, err := s.accountTarget(ctx, q, actor, id)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSdm {
			return errAccountChange("A2F wajib hanya bisa diatur untuk akun SDM. Super admin selalu wajib A2F.")
		}
		if user.MfaRequired == required {
			return nil
		}
		if err := q.SetUserMFARequired(ctx, store.SetUserMFARequiredParams{ID: user.ID, MfaRequired: required}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(actor.UserID), Action: audit.UserMFAPolicyChanged, Entity: "user", EntityID: user.ID, IP: ip,
			Detail: map[string]any{"required": required},
		})
	})
}

type roleRequest struct {
	Role string `json:"role"`
}

// AdminSetRole melayani PUT /v1/admin/users/{user_id}/role.
func (h *Handler) AdminSetRole(w http.ResponseWriter, r *http.Request) error {
	p, _ := PrincipalFrom(r.Context())
	id, err := pathUUID(r, "user_id")
	if err != nil {
		return err
	}
	var req roleRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.svc.SetAccountRole(r.Context(), p, id, req.Role, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

type mfaPolicyRequest struct {
	Required *bool `json:"required"`
}

// AdminSetMFAPolicy melayani PUT /v1/admin/users/{user_id}/mfa-policy.
func (h *Handler) AdminSetMFAPolicy(w http.ResponseWriter, r *http.Request) error {
	p, _ := PrincipalFrom(r.Context())
	id, err := pathUUID(r, "user_id")
	if err != nil {
		return err
	}
	var req mfaPolicyRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.Required == nil {
		return httpx.Validation(map[string]string{"required": "Isi true atau false."})
	}
	if err := h.svc.SetMFAPolicy(r.Context(), p, id, *req.Required, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

// resetMFA menghapus A2F akun dan mencabut semua sesinya (D71). Bila A2F wajib, akun mendaftarkan
// authenticator baru saat login berikutnya.
func resetMFA(ctx context.Context, q *store.Queries, user store.User, entry audit.Entry, source string) error {
	n, err := q.DeleteUserMFA(ctx, user.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return errAccountChange("Akun ini belum memakai A2F.")
	}
	if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "mfa_reset", Valid: true}}); err != nil {
		return err
	}
	entry.Action, entry.Entity, entry.EntityID = audit.UserMFAReset, "user", user.ID
	entry.Detail = map[string]any{"source": source}
	return audit.Write(ctx, q, entry)
}

// ResetMFA menghapus A2F akun SDM atau super admin lain, misalnya karena HP authenticator hilang (D71).
// A2F akun sendiri direset lewat perintah server hris-superadmin reset-mfa.
func (s *Service) ResetMFA(ctx context.Context, actor Principal, id uuid.UUID, ip *netip.Addr) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if id == actor.UserID {
			return errSelfAction("A2F akun sendiri direset super admin lain atau lewat perintah di server.")
		}
		user, err := s.accountTarget(ctx, q, actor, id)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSdm && user.Role != store.UserRoleSuperadmin {
			return errAccountChange("Reset A2F hanya untuk akun SDM dan super admin.")
		}
		return resetMFA(ctx, q, user, audit.Entry{UserID: audit.Actor(actor.UserID), IP: ip}, "panel")
	})
}

// AdminResetMFA melayani DELETE /v1/admin/users/{user_id}/mfa.
func (h *Handler) AdminResetMFA(w http.ResponseWriter, r *http.Request) error {
	p, _ := PrincipalFrom(r.Context())
	id, err := pathUUID(r, "user_id")
	if err != nil {
		return err
	}
	if err := h.svc.ResetMFA(r.Context(), p, id, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
