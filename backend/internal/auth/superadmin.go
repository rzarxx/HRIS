package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// changeRole mengganti peran akun. source adalah "cli" (perintah server) atau "panel" (super admin, D60).
func changeRole(ctx context.Context, q *store.Queries, user store.User, to store.UserRole, entry audit.Entry, source string) error {
	if err := q.SetUserRole(ctx, store.SetUserRoleParams{Role: to, ID: user.ID}); err != nil {
		return err
	}
	// Peran ada di klaim token, jadi sesi lama dicabut agar peran baru langsung berlaku.
	if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "role_changed", Valid: true}}); err != nil {
		return err
	}
	entry.Action, entry.Entity, entry.EntityID = audit.UserRoleChanged, "user", user.ID
	entry.Detail = map[string]any{"from": string(user.Role), "to": string(to), "source": source}
	return audit.Write(ctx, q, entry)
}

// userByIdentifier mencari akun dari NIPY, email, atau nomor WA, sama dengan login (D53).
func userByIdentifier(ctx context.Context, q *store.Queries, identifier string) (store.User, error) {
	row, err := lookupLoginUser(ctx, q, identifier)
	if pgconv.IsNoRows(err) {
		return store.User{}, fmt.Errorf("akun %q tidak ditemukan", identifier)
	}
	if err != nil {
		return store.User{}, err
	}
	return q.GetUserByID(ctx, row.ID)
}

// resetToInitialPassword memasang kata sandi awal (D53) dan mencabut semua sesi akun.
func resetToInitialPassword(ctx context.Context, q *store.Queries, user store.User, now time.Time) (InitialCredential, error) {
	me, err := q.GetMe(ctx, user.ID)
	if err != nil {
		return InitialCredential{}, err
	}
	out, hash, err := InitialPasswordHash(me.FullName, now)
	if err != nil {
		return InitialCredential{}, err
	}
	if err := q.SetInitialPassword(ctx, store.SetInitialPasswordParams{ID: user.ID, PasswordHash: hash, InitialPasswordExpiresAt: out.Expires()}); err != nil {
		return InitialCredential{}, err
	}
	if err := q.InvalidateUserInvites(ctx, user.ID); err != nil {
		return InitialCredential{}, err
	}
	if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "password_reset", Valid: true}}); err != nil {
		return InitialCredential{}, err
	}
	return out, nil
}

// GrantSuperadmin menjadikan akun Pejuang biasa sebagai super admin. Hanya lewat perintah
// di server, karena tidak ada endpoint ubah role (D25). Akun lama yang belum pernah aktif
// diaktifkan dengan kata sandi awal yang wajib diganti saat login pertama.
func GrantSuperadmin(ctx context.Context, pool *pgxpool.Pool, identifier string, now time.Time) (*InitialCredential, error) {
	var credential *InitialCredential
	err := database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByIdentifier(ctx, q, identifier)
		if err != nil {
			return err
		}
		status, err := q.GetEmployeeStatus(ctx, user.EmployeeID)
		if err != nil {
			return err
		}
		if status != "active" {
			return errors.New("Pejuang ini sudah nonaktif")
		}
		switch user.Role {
		case store.UserRoleSuperadmin:
		case store.UserRolePejuang:
			if err := changeRole(ctx, q, user, store.UserRoleSuperadmin, audit.Entry{}, "cli"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("akun ber-role %s tidak bisa dijadikan super admin; pakai akun Pejuang biasa", user.Role)
		}
		if !user.IsActive {
			out, err := resetToInitialPassword(ctx, q, user, now)
			if err != nil {
				return err
			}
			if err := q.SetUserActive(ctx, store.SetUserActiveParams{ID: user.ID, IsActive: true}); err != nil {
				return err
			}
			credential = &out
		}
		return nil
	})
	return credential, err
}

// ResetSuperadmin mengembalikan super admin ke kata sandi awal dari server. Hanya sesama super admin
// yang bisa melakukannya lewat web, jadi ini jalan keluar bila super admin satu-satunya lupa sandi.
func ResetSuperadmin(ctx context.Context, pool *pgxpool.Pool, identifier string, now time.Time) (InitialCredential, error) {
	var out InitialCredential
	err := database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByIdentifier(ctx, q, identifier)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSuperadmin {
			return fmt.Errorf("akun ini ber-role %s, bukan super admin", user.Role)
		}
		if out, err = resetToInitialPassword(ctx, q, user, now); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			Action: audit.UserPasswordReset, Entity: "user", EntityID: user.ID, Detail: map[string]any{"source": "cli"},
		})
	})
	return out, err
}

// RevokeSuperadmin mengembalikan super admin menjadi Pejuang biasa.
func RevokeSuperadmin(ctx context.Context, pool *pgxpool.Pool, identifier string) error {
	return database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByIdentifier(ctx, q, identifier)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSuperadmin {
			return fmt.Errorf("akun ini ber-role %s, bukan super admin", user.Role)
		}
		return changeRole(ctx, q, user, store.UserRolePejuang, audit.Entry{}, "cli")
	})
}

// ResetSuperadminMFA menghapus A2F super admin lewat perintah server, untuk super admin yang kehilangan
// authenticator dan tidak ada super admin lain (D71).
func ResetSuperadminMFA(ctx context.Context, pool *pgxpool.Pool, identifier string) error {
	return database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByIdentifier(ctx, q, identifier)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSuperadmin {
			return fmt.Errorf("akun ini ber-role %s, bukan super admin", user.Role)
		}
		return resetMFA(ctx, q, user, audit.Entry{}, "cli")
	})
}
