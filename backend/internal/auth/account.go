package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	inviteTTL        = 24 * time.Hour
	minPasswordRunes = 10
	maxPasswordBytes = 256
	recoveryCount    = 8
	// Akun yang dikunci SDM dibuka manual; nilai ini sekadar "sangat lama".
	adminLockDuration = 100 * 365 * 24 * time.Hour
)

// Daftar singkat sandi umum (MASTER.md §5.2). Dibandingkan tanpa peduli huruf besar-kecil.
var commonPasswords = map[string]bool{
	"1234567890": true, "12345678910": true, "123456789012": true, "0123456789": true, "0987654321": true,
	"1111111111": true, "password123": true, "password1234": true, "passw0rd123": true, "qwertyuiop": true,
	"qwerty12345": true, "qwerty123456": true, "1q2w3e4r5t": true, "1qaz2wsx3edc": true, "iloveyou123": true,
	"admin12345": true, "admin123456": true, "administrator": true, "letmein1234": true, "welcome123": true,
	"pejuang123": true, "pejuang1234": true, "indonesia123": true, "indonesia1945": true, "bismillah123": true,
	"sayang12345": true, "rahasia123": true, "katasandi123": true,
}

// ValidatePassword mengembalikan pesan untuk pengguna; kosong berarti lolos.
func ValidatePassword(password, email string) string {
	switch {
	case utf8.RuneCountInString(password) < minPasswordRunes:
		return fmt.Sprintf("Kata sandi minimal %d karakter.", minPasswordRunes)
	case len(password) > maxPasswordBytes:
		return "Kata sandi melebihi panjang maksimal."
	case commonPasswords[strings.ToLower(password)]:
		return "Kata sandi ini mudah ditebak. Silakan pilih yang lain."
	case email != "" && strings.EqualFold(password, email):
		return "Kata sandi perlu berbeda dari email."
	}
	return ""
}

// InitialCredential adalah kata sandi awal yang ditampilkan ke SDM setelah akun dibuat atau
// direset (D53). Menggantikan tautan aktivasi.
type InitialCredential struct {
	InitialPassword string `json:"initial_password"`
	ExpiresAt       string `json:"expires_at"`
	expires         time.Time
}

// Expires adalah batas berlaku kata sandi awal untuk disimpan di users.initial_password_expires_at.
func (c InitialCredential) Expires() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: c.expires, Valid: true}
}

// InitialPasswordHash membuat kata sandi awal beserta hash-nya. Argon2 sengaja lambat, jadi
// pemanggil menghitungnya di luar transaksi.
func InitialPasswordHash(fullName string, now time.Time) (InitialCredential, string, error) {
	credential := NewInitialCredential(fullName, now)
	hash, err := HashInitialPassword(credential.InitialPassword)
	return credential, hash, err
}

// NewInitialCredential menyusun kata sandi awal dan batas berlakunya tanpa meng-hash, untuk impor
// massal yang meng-hash banyak kata sandi sekaligus.
func NewInitialCredential(fullName string, now time.Time) InitialCredential {
	expires := now.Add(InitialPasswordTTL)
	return InitialCredential{InitialPassword: InitialPassword(fullName, now), ExpiresAt: expires.UTC().Format(time.RFC3339), expires: expires}
}

// ChangePassword memeriksa sandi lama dan mencabut sesi lain.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string, ip *netip.Addr) error {
	user, err := s.q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(current, user.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation(map[string]string{"current_password": "Kata sandi saat ini belum sesuai. Silakan periksa kembali."})
	}
	if msg := ValidatePassword(next, user.Email.String); msg != "" {
		return httpx.Validation(map[string]string{"new_password": msg})
	}
	if current == next {
		return httpx.Validation(map[string]string{"new_password": "Kata sandi baru perlu berbeda dari yang lama."})
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.SetUserPassword(ctx, store.SetUserPasswordParams{ID: p.UserID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.RevokeOtherSessions(ctx, store.RevokeOtherSessionsParams{
			UserID: p.UserID, KeepSessionID: p.SessionID, Reason: pgtype.Text{String: "password_changed", Valid: true},
		}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AuthPasswordChanged, Entity: "user", EntityID: p.UserID, IP: ip,
		})
	})
	if err != nil {
		return err
	}
	// Kata sandi awal yang diganti saat login pertama bukan perubahan yang perlu diwaspadai.
	if !user.MustChangePassword {
		s.notifySecurity(ctx, p.UserID, "password_changed", "Kata sandi diganti")
	}
	return nil
}

// SessionView adalah sesi aktif untuk ditampilkan ke pemiliknya.
type SessionView struct {
	ID         string  `json:"id"`
	Client     string  `json:"client"`
	IP         *string `json:"ip"`
	UserAgent  *string `json:"user_agent"`
	CreatedAt  *string `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	IsCurrent  bool    `json:"is_current"`
}

func (s *Service) ListSessions(ctx context.Context, p Principal) ([]SessionView, error) {
	rows, err := s.q.ListActiveSessions(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(rows))
	for _, r := range rows {
		var ip *string
		if r.Ip != nil {
			v := r.Ip.String()
			ip = &v
		}
		out = append(out, SessionView{
			ID: r.ID.String(), Client: r.Client, IP: ip, UserAgent: pgconv.TextPtr(r.UserAgent),
			CreatedAt: pgconv.TimeString(r.CreatedAt), LastUsedAt: pgconv.TimeString(r.LastUsedAt),
			IsCurrent: r.ID == p.SessionID,
		})
	}
	return out, nil
}

// RevokeOwnSession menjawab 404 untuk sesi milik orang lain.
func (s *Service) RevokeOwnSession(ctx context.Context, p Principal, id uuid.UUID, ip *netip.Addr) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.RevokeOwnSession(ctx, store.RevokeOwnSessionParams{ID: id, UserID: p.UserID})
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AuthSessionRevoked, Entity: "session", EntityID: id, IP: ip,
		})
	})
}

// MFASetup adalah secret TOTP untuk didaftarkan di aplikasi authenticator.
type MFASetup struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
}

func errMFAAlreadyEnabled() error {
	return httpx.NewError(http.StatusConflict, "MFA_ALREADY_ENABLED", "MFA sudah aktif untuk akun ini.")
}

func (s *Service) SetupMFA(ctx context.Context, p Principal) (MFASetup, error) {
	user, err := s.q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return MFASetup{}, err
	}
	existing, err := s.q.GetUserMfa(ctx, p.UserID)
	if err != nil && !pgconv.IsNoRows(err) {
		return MFASetup{}, err
	}
	if err == nil && existing.EnabledAt.Valid {
		return MFASetup{}, errMFAAlreadyEnabled()
	}
	secret, err := NewTOTPSecret()
	if err != nil {
		return MFASetup{}, err
	}
	enc, err := s.fieldCipher.Encrypt([]byte(secret))
	if err != nil {
		return MFASetup{}, err
	}
	if err := s.q.UpsertPendingMfa(ctx, store.UpsertPendingMfaParams{UserID: p.UserID, SecretEnc: enc}); err != nil {
		return MFASetup{}, err
	}
	label := user.Email.String
	if label == "" {
		me, err := s.q.GetMe(ctx, p.UserID)
		if err != nil {
			return MFASetup{}, err
		}
		label = me.Nip
	}
	return MFASetup{Secret: secret, OtpauthURL: TOTPURL(secret, label)}, nil
}

// EnableMFA mengaktifkan TOTP dan mengembalikan kode pemulihan sekali saja.
func (s *Service) EnableMFA(ctx context.Context, p Principal, code string, ip *netip.Addr, now time.Time) ([]string, error) {
	mfa, err := s.q.GetUserMfa(ctx, p.UserID)
	if pgconv.IsNoRows(err) {
		return nil, httpx.Validation(map[string]string{"code": "Mulai pendaftaran MFA terlebih dahulu."})
	}
	if err != nil {
		return nil, err
	}
	if mfa.EnabledAt.Valid {
		return nil, errMFAAlreadyEnabled()
	}
	secret, err := s.fieldCipher.Decrypt(mfa.SecretEnc)
	if err != nil {
		return nil, err
	}
	step, ok := VerifyTOTP(string(secret), code, now, 0)
	if !ok {
		return nil, errMFAInvalid()
	}
	codes, hashes, err := NewRecoveryCodes(recoveryCount)
	if err != nil {
		return nil, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.EnableMfa(ctx, store.EnableMfaParams{UserID: p.UserID, RecoveryHashes: hashes, LastUsedStep: step}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AuthMFAEnabled, Entity: "user", EntityID: p.UserID, IP: ip,
		})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func errMFAInvalid() error {
	return httpx.NewError(http.StatusUnauthorized, "AUTH_MFA_INVALID", "Kode belum sesuai. Silakan periksa kembali.")
}

// checkMFACode menerima kode TOTP atau kode pemulihan.
func (s *Service) checkMFACode(ctx context.Context, userID uuid.UUID, code string, now time.Time) error {
	mfa, err := s.q.GetUserMfa(ctx, userID)
	if pgconv.IsNoRows(err) || (err == nil && !mfa.EnabledAt.Valid) {
		return errMFAInvalid()
	}
	if err != nil {
		return err
	}
	secret, err := s.fieldCipher.Decrypt(mfa.SecretEnc)
	if err != nil {
		return err
	}
	if step, ok := VerifyTOTP(string(secret), code, now, mfa.LastUsedStep); ok {
		affected, err := s.q.SetMfaLastStep(ctx, store.SetMfaLastStepParams{UserID: userID, Step: step})
		if err != nil {
			return err
		}
		if affected == 0 {
			return errMFAInvalid() // kalah balapan dengan permintaan lain yang memakai kode sama
		}
		return nil
	}
	if rest, ok := ConsumeRecoveryCode(mfa.RecoveryHashes, code); ok {
		return s.q.SetMfaRecoveryHashes(ctx, store.SetMfaRecoveryHashesParams{UserID: userID, RecoveryHashes: rest})
	}
	return errMFAInvalid()
}

// --- Kelola akun oleh SDM (permission user:manage) ---

func (s *Service) managedUser(ctx context.Context, id uuid.UUID) (store.User, error) {
	user, err := s.q.GetUserByID(ctx, id)
	if pgconv.IsNoRows(err) {
		return user, httpx.ErrNotFound()
	}
	return user, err
}

// Super admin adalah role tertinggi, jadi SDM tidak bisa mengubah akunnya (D34).
func (s *Service) managedTarget(ctx context.Context, actor Principal, id uuid.UUID) (store.User, error) {
	user, err := s.managedUser(ctx, id)
	if err != nil {
		return user, err
	}
	if actor.Role != permission.RoleSuperadmin && user.Role == store.UserRoleSuperadmin {
		return user, httpx.ErrForbidden()
	}
	return user, nil
}

// AssertEmployeeNotProtected menolak perubahan data Pejuang yang akunnya super admin, kecuali
// oleh super admin sendiri (D34).
func AssertEmployeeNotProtected(ctx context.Context, q *store.Queries, actor Principal, employeeID uuid.UUID) error {
	if actor.Role == permission.RoleSuperadmin {
		return nil
	}
	user, err := q.GetUserByEmployee(ctx, employeeID)
	if pgconv.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.Role == store.UserRoleSuperadmin {
		return httpx.ErrForbidden()
	}
	return nil
}

func errSelfAction(message string) error {
	return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", message)
}

// ResetPassword mengembalikan akun ke kata sandi awal (D53) yang wajib diganti saat login berikutnya,
// dan mencabut semua sesinya.
func (s *Service) ResetPassword(ctx context.Context, actor Principal, id uuid.UUID, ip *netip.Addr, now time.Time) (InitialCredential, error) {
	if id == actor.UserID {
		return InitialCredential{}, errSelfAction("Gunakan menu ganti kata sandi untuk akun sendiri.")
	}
	user, err := s.managedTarget(ctx, actor, id)
	if err != nil {
		return InitialCredential{}, err
	}
	// Kata sandi tampil ke yang menekan tombol, dan akun SDM memegang data keuangan (D33).
	if actor.Role == permission.RoleSuperadmin && user.Role == store.UserRoleSdm {
		return InitialCredential{}, httpx.ErrForbidden()
	}
	me, err := s.q.GetMe(ctx, id)
	if err != nil {
		return InitialCredential{}, err
	}
	out, hash, err := InitialPasswordHash(me.FullName, now)
	if err != nil {
		return InitialCredential{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.SetInitialPassword(ctx, store.SetInitialPasswordParams{ID: id, PasswordHash: hash, InitialPasswordExpiresAt: out.Expires()}); err != nil {
			return err
		}
		// Akun lama yang belum pernah aktivasi (sebelum D53) ikut diaktifkan, kecuali Pejuangnya sudah nonaktif.
		if !user.IsActive {
			status, err := q.GetEmployeeStatus(ctx, user.EmployeeID)
			if err != nil {
				return err
			}
			if status == "active" {
				if err := q.SetUserActive(ctx, store.SetUserActiveParams{ID: id, IsActive: true}); err != nil {
					return err
				}
			}
		}
		if err := q.InvalidateUserInvites(ctx, id); err != nil {
			return err
		}
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: id, Reason: pgtype.Text{String: "password_reset", Valid: true}}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(actor.UserID), Action: audit.UserPasswordReset, Entity: "user", EntityID: id, IP: ip,
		})
	})
	return out, err
}

func (s *Service) LockUser(ctx context.Context, actor Principal, id uuid.UUID, ip *netip.Addr, now time.Time) error {
	if id == actor.UserID {
		return errSelfAction("Anda tidak dapat mengunci akun sendiri.")
	}
	if _, err := s.managedTarget(ctx, actor, id); err != nil {
		return err
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.SetUserLock(ctx, store.SetUserLockParams{ID: id, LockedUntil: pgconv.Time(now.Add(adminLockDuration))}); err != nil {
			return err
		}
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: id, Reason: pgtype.Text{String: "locked", Valid: true}}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(actor.UserID), Action: audit.UserLocked, Entity: "user", EntityID: id, IP: ip,
		})
	})
}

func (s *Service) UnlockUser(ctx context.Context, actor Principal, id uuid.UUID, ip *netip.Addr) error {
	if _, err := s.managedTarget(ctx, actor, id); err != nil {
		return err
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.SetUserLock(ctx, store.SetUserLockParams{ID: id}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(actor.UserID), Action: audit.UserUnlocked, Entity: "user", EntityID: id, IP: ip,
		})
	})
}

func (s *Service) RevokeUserSessions(ctx context.Context, actor Principal, id uuid.UUID, ip *netip.Addr) error {
	if id == actor.UserID {
		return errSelfAction("Gunakan menu sesi aktif untuk akun sendiri.")
	}
	if _, err := s.managedTarget(ctx, actor, id); err != nil {
		return err
	}
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: id, Reason: pgtype.Text{String: "admin_revoked", Valid: true}}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(actor.UserID), Action: audit.AuthSessionRevoked, Entity: "user", EntityID: id, IP: ip,
		})
	})
}
