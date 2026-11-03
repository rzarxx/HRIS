package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

func changeRole(ctx context.Context, q *store.Queries, user store.User, to store.UserRole) error {
	if err := q.SetUserRole(ctx, store.SetUserRoleParams{Role: to, ID: user.ID}); err != nil {
		return err
	}
	// Peran ada di klaim token, jadi sesi lama dicabut agar peran baru langsung berlaku.
	if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "role_changed", Valid: true}}); err != nil {
		return err
	}
	return audit.Write(ctx, q, audit.Entry{
		Action: audit.UserRoleChanged, Entity: "user", EntityID: user.ID,
		Detail: map[string]any{"from": string(user.Role), "to": string(to), "source": "cli"},
	})
}

func userByEmail(ctx context.Context, q *store.Queries, email string) (store.User, error) {
	row, err := q.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if pgconv.IsNoRows(err) {
		return store.User{}, fmt.Errorf("akun dengan email %q tidak ditemukan", email)
	}
	if err != nil {
		return store.User{}, err
	}
	return q.GetUserByID(ctx, row.ID)
}

// GrantSuperadmin menjadikan akun Pejuang biasa sebagai super admin. Hanya lewat perintah
// di server, karena tidak ada endpoint ubah role (D25). Bila akun belum diaktivasi, undangan
// aktivasi dikembalikan agar akun pertama tidak bergantung pada SMTP yang belum diatur.
func GrantSuperadmin(ctx context.Context, pool *pgxpool.Pool, email string, now time.Time) (*Activation, error) {
	var activation *Activation
	err := database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByEmail(ctx, q, email)
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
			if err := changeRole(ctx, q, user, store.UserRoleSuperadmin); err != nil {
				return err
			}
		default:
			return fmt.Errorf("akun ber-role %s tidak bisa dijadikan super admin; pakai akun Pejuang biasa", user.Role)
		}
		if !user.IsActive {
			invite, err := IssueInvite(ctx, q, user.ID, now)
			if err != nil {
				return err
			}
			activation = &invite
		}
		return nil
	})
	return activation, err
}

// ResetSuperadmin menerbitkan tautan sandi baru dari server. Hanya sesama super admin yang
// bisa melakukannya lewat web, jadi ini jalan keluar bila super admin satu-satunya lupa sandi.
func ResetSuperadmin(ctx context.Context, pool *pgxpool.Pool, email string, now time.Time) (Activation, error) {
	hash, err := UnusablePasswordHash()
	if err != nil {
		return Activation{}, err
	}
	var out Activation
	err = database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByEmail(ctx, q, email)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSuperadmin {
			return fmt.Errorf("akun ini ber-role %s, bukan super admin", user.Role)
		}
		if err := q.SetUserPassword(ctx, store.SetUserPasswordParams{ID: user.ID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "password_reset", Valid: true}}); err != nil {
			return err
		}
		if out, err = IssueInvite(ctx, q, user.ID, now); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			Action: audit.UserPasswordReset, Entity: "user", EntityID: user.ID, Detail: map[string]any{"source": "cli"},
		})
	})
	return out, err
}

// RevokeSuperadmin mengembalikan super admin menjadi Pejuang biasa.
func RevokeSuperadmin(ctx context.Context, pool *pgxpool.Pool, email string) error {
	return database.InTx(ctx, pool, func(q *store.Queries) error {
		user, err := userByEmail(ctx, q, email)
		if err != nil {
			return err
		}
		if user.Role != store.UserRoleSuperadmin {
			return fmt.Errorf("akun ini ber-role %s, bukan super admin", user.Role)
		}
		return changeRole(ctx, q, user, store.UserRolePejuang)
	})
}
