package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	maxLoginAttempts = 5
	lockoutDuration  = 15 * time.Minute
	webIdleTTL       = 30 * time.Minute
	webAbsoluteTTL   = 12 * time.Hour
)

// Service memuat aturan login dan sesi (MASTER.md §5.2, F4, F6).
type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	signer *TokenSigner
	// fieldCipher mengenkripsi secret TOTP.
	fieldCipher *fieldcrypt.Cipher
	// requireMFA mewajibkan MFA untuk role sdm (MASTER.md §5.2).
	requireMFA bool
	// dummyHash dipakai saat email tidak ada agar waktu respons tidak membocorkan keberadaan akun.
	dummyHash string
}

func NewService(pool *pgxpool.Pool, signer *TokenSigner, fieldCipher *fieldcrypt.Cipher, requireMFA bool) (*Service, error) {
	dummy, err := HashPassword("dummy-untuk-menyamakan-waktu")
	if err != nil {
		return nil, err
	}
	return &Service{pool: pool, q: store.New(pool), signer: signer, fieldCipher: fieldCipher, requireMFA: requireMFA, dummyHash: dummy}, nil
}

// Tokens adalah hasil login atau refresh, ditulis ke cookie oleh handler.
type Tokens struct {
	Access  string
	Refresh string
	CSRF    string
}

func errInvalidCredentials() error {
	return httpx.NewError(http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "Email atau kata sandi salah.")
}

// Login mengembalikan user ID dan token. Pesan gagal selalu generik.
func (s *Service) Login(ctx context.Context, email, password string, ip *netip.Addr, userAgent string, now time.Time) (uuid.UUID, Tokens, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if pgconv.IsNoRows(err) {
		_, _ = VerifyPassword(password, s.dummyHash)
		s.auditBestEffort(ctx, audit.Entry{Action: audit.AuthLoginFailed, IP: ip})
		return uuid.Nil, Tokens{}, errInvalidCredentials()
	}
	if err != nil {
		return uuid.Nil, Tokens{}, err
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(now) {
		return uuid.Nil, Tokens{}, &httpx.Error{
			Status:  http.StatusLocked,
			Code:    "AUTH_ACCOUNT_LOCKED",
			Message: "Akun terkunci sementara. Coba lagi nanti.",
			Details: map[string]any{"retry_after_seconds": int(user.LockedUntil.Time.Sub(now).Seconds()) + 1},
		}
	}

	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return uuid.Nil, Tokens{}, err
	}
	if !ok || !user.IsActive {
		if err := s.registerFailure(ctx, user.ID, ip, now); err != nil {
			return uuid.Nil, Tokens{}, err
		}
		return uuid.Nil, Tokens{}, errInvalidCredentials()
	}

	// MFA aktif: sesi baru dibuat setelah kode TOTP benar.
	mfa, err := s.q.GetUserMfa(ctx, user.ID)
	if err != nil && !pgconv.IsNoRows(err) {
		return uuid.Nil, Tokens{}, err
	}
	if err == nil && mfa.EnabledAt.Valid {
		mfaToken, err := s.signer.SignMFA(user.ID, now)
		if err != nil {
			return uuid.Nil, Tokens{}, err
		}
		return uuid.Nil, Tokens{}, &httpx.Error{
			Status:  http.StatusUnauthorized,
			Code:    "AUTH_MFA_REQUIRED",
			Message: "Masukkan kode dari aplikasi authenticator.",
			Details: map[string]any{"mfa_token": mfaToken},
		}
	}
	tokens, err := s.startSession(ctx, user.ID, user.EmployeeID, string(user.Role), ip, userAgent, now)
	if err != nil {
		return uuid.Nil, Tokens{}, err
	}
	return user.ID, tokens, nil
}

// VerifyMFA menyelesaikan login dari token sementara dan kode TOTP atau kode pemulihan.
func (s *Service) VerifyMFA(ctx context.Context, mfaToken, code string, ip *netip.Addr, userAgent string, now time.Time) (uuid.UUID, Tokens, error) {
	userID, err := s.signer.VerifyMFA(mfaToken)
	if err != nil {
		return uuid.Nil, Tokens{}, httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_INVALID", "Waktu verifikasi habis, silakan masuk kembali.")
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return uuid.Nil, Tokens{}, err
	}
	if !user.IsActive || (user.LockedUntil.Valid && user.LockedUntil.Time.After(now)) {
		return uuid.Nil, Tokens{}, errInvalidCredentials()
	}
	if err := s.checkMFACode(ctx, userID, code, now); err != nil {
		s.auditBestEffort(ctx, audit.Entry{UserID: audit.Actor(userID), Action: audit.AuthMFAFailed, Entity: "user", EntityID: userID, IP: ip})
		return uuid.Nil, Tokens{}, err
	}
	tokens, err := s.startSession(ctx, userID, user.EmployeeID, string(user.Role), ip, userAgent, now)
	if err != nil {
		return uuid.Nil, Tokens{}, err
	}
	return userID, tokens, nil
}

func (s *Service) startSession(ctx context.Context, userID, employeeID uuid.UUID, role string, ip *netip.Addr, userAgent string, now time.Time) (Tokens, error) {
	tokens := Tokens{}
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.RegisterLoginSuccess(ctx, userID); err != nil {
			return err
		}
		refresh, err := NewOpaqueToken()
		if err != nil {
			return err
		}
		ua := pgtype.Text{}
		if userAgent != "" {
			ua = pgtype.Text{String: truncate(userAgent, 256), Valid: true}
		}
		sessionID, err := q.CreateSession(ctx, store.CreateSessionParams{
			UserID:            userID,
			Client:            "web",
			RefreshHash:       HashToken(refresh),
			Ip:                ip,
			UserAgent:         ua,
			IdleExpiresAt:     pgconv.Time(now.Add(webIdleTTL)),
			AbsoluteExpiresAt: pgconv.Time(now.Add(webAbsoluteTTL)),
		})
		if err != nil {
			return err
		}
		if tokens, err = s.issue(userID, employeeID, sessionID, role, refresh, now); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(userID), Action: audit.AuthLoginSuccess, Entity: "session", EntityID: sessionID, IP: ip,
		})
	})
	return tokens, err
}

func (s *Service) registerFailure(ctx context.Context, userID uuid.UUID, ip *netip.Addr, now time.Time) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.RegisterLoginFailure(ctx, store.RegisterLoginFailureParams{
			ID:          userID,
			MaxAttempts: maxLoginAttempts,
			LockedUntil: pgconv.Time(now.Add(lockoutDuration)),
		})
		if err != nil {
			return err
		}
		if err := audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(userID), Action: audit.AuthLoginFailed, Entity: "user", EntityID: userID, IP: ip,
		}); err != nil {
			return err
		}
		if row.FailedLoginCount == maxLoginAttempts {
			return audit.Write(ctx, q, audit.Entry{
				UserID: audit.Actor(userID), Action: audit.UserLocked, Entity: "user", EntityID: userID, IP: ip,
			})
		}
		return nil
	})
}

// Refresh merotasi refresh token. Token lama yang dipakai ulang mencabut sesi (F6).
func (s *Service) Refresh(ctx context.Context, refresh string, ip *netip.Addr, now time.Time) (Tokens, error) {
	if refresh == "" {
		return Tokens{}, httpx.ErrUnauthenticated()
	}
	hash := HashToken(refresh)
	sess, err := s.q.GetSessionByRefreshHash(ctx, hash)
	if pgconv.IsNoRows(err) {
		return Tokens{}, s.handleReuse(ctx, hash, ip)
	}
	if err != nil {
		return Tokens{}, err
	}
	if sess.Client != "web" || sess.RevokedAt.Valid || !sess.IsActive ||
		now.After(sess.IdleExpiresAt.Time) || now.After(sess.AbsoluteExpiresAt.Time) {
		return Tokens{}, httpx.ErrUnauthenticated()
	}

	next, err := NewOpaqueToken()
	if err != nil {
		return Tokens{}, err
	}
	idle := now.Add(webIdleTTL)
	if idle.After(sess.AbsoluteExpiresAt.Time) {
		idle = sess.AbsoluteExpiresAt.Time
	}
	if err := s.q.RotateSession(ctx, store.RotateSessionParams{
		ID: sess.ID, NewHash: HashToken(next), IdleExpiresAt: pgconv.Time(idle),
	}); err != nil {
		return Tokens{}, err
	}
	return s.issue(sess.UserID, sess.EmployeeID, sess.ID, string(sess.Role), next, now)
}

func (s *Service) handleReuse(ctx context.Context, hash string, ip *netip.Addr) error {
	prev, err := s.q.GetSessionByPrevRefreshHash(ctx, store.GetSessionByPrevRefreshHashParams{PrevRefreshHash: pgtype.Text{String: hash, Valid: true}, Client: "web"})
	if pgconv.IsNoRows(err) {
		return httpx.ErrUnauthenticated()
	}
	if err != nil {
		return err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.RevokeSession(ctx, store.RevokeSessionParams{
			ID: prev.ID, RevokeReason: pgtype.Text{String: "refresh_reuse", Valid: true},
		}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(prev.UserID), Action: audit.AuthRefreshReuse, Entity: "session", EntityID: prev.ID, IP: ip,
		})
	})
	if err != nil {
		return err
	}
	return httpx.ErrUnauthenticated()
}

// Logout mencabut sesi dari access token yang sudah diverifikasi middleware.
func (s *Service) Logout(ctx context.Context, p Principal, ip *netip.Addr) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.RevokeSession(ctx, store.RevokeSessionParams{
			ID: p.SessionID, RevokeReason: pgtype.Text{String: "logout", Valid: true},
		}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AuthLogout, Entity: "session", EntityID: p.SessionID, IP: ip,
		})
	})
}

// SessionState dipakai middleware agar sesi yang dicabut langsung mati.
func (s *Service) SessionState(ctx context.Context, sessionID uuid.UUID) (SessionState, error) {
	row, err := s.q.GetSessionAuthState(ctx, sessionID)
	if pgconv.IsNoRows(err) {
		return SessionState{}, nil
	}
	if err != nil {
		return SessionState{}, err
	}
	return SessionState{
		Active:             row.IsActive,
		MustChangePassword: row.MustChangePassword,
		MFASetupRequired:   s.mfaSetupRequired(string(row.Role), row.MfaEnabled),
		UserID:             row.UserID,
		EmployeeID:         row.EmployeeID,
		Role:               string(row.Role),
		Client:             row.Client,
		DeviceID:           row.DeviceID,
		DeviceActive:       row.DeviceActive,
	}, nil
}

func (s *Service) mfaSetupRequired(role string, enabled bool) bool {
	return s.requireMFA && permission.RequiresMFA(role) && !enabled
}

// Me membentuk objek baku GET /v1/me (MASTER.md §7.6).
type Me struct {
	UserID             string   `json:"user_id"`
	EmployeeID         string   `json:"employee_id"`
	Role               string   `json:"role"`
	FullName           string   `json:"full_name"`
	Nip                string   `json:"nip"`
	Email              string   `json:"email"`
	Position           *string  `json:"position"`
	Subdivision        Ref      `json:"subdivision"`
	Division           Ref      `json:"division"`
	Permissions        []string `json:"permissions"`
	MustChangePassword bool     `json:"must_change_password"`
	MFAEnabled         bool     `json:"mfa_enabled"`
}

type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (Me, error) {
	row, err := s.q.GetMe(ctx, userID)
	if pgconv.IsNoRows(err) {
		return Me{}, httpx.ErrUnauthenticated()
	}
	if err != nil {
		return Me{}, err
	}
	role := string(row.Role)
	mfa, err := s.q.GetUserMfa(ctx, userID)
	if err != nil && !pgconv.IsNoRows(err) {
		return Me{}, err
	}
	mfaEnabled := err == nil && mfa.EnabledAt.Valid
	return Me{
		UserID:             row.UserID.String(),
		EmployeeID:         row.EmployeeID.String(),
		Role:               role,
		FullName:           row.FullName,
		Nip:                row.Nip,
		Email:              row.Email,
		Position:           pgconv.TextPtr(row.Position),
		Subdivision:        Ref{ID: row.SubdivisionID.String(), Name: row.SubdivisionName},
		Division:           Ref{ID: row.DivisionID.String(), Name: row.DivisionName},
		Permissions:        permission.Keys(role),
		MustChangePassword: row.MustChangePassword,
		MFAEnabled:         mfaEnabled,
	}, nil
}

// ScopeFilter menyatakan data yang boleh dilihat seseorang; dipakai query bertanda scope.
type ScopeFilter struct {
	All            bool
	SubdivisionIDs []uuid.UUID
	DivisionIDs    []uuid.UUID
}

// ResolveScope dihitung dari struktur organisasi di DB, bukan dari klaim JWT (MASTER.md §3.1).
func (s *Service) ResolveScope(ctx context.Context, p Principal, key string) (ScopeFilter, error) {
	head := uuid.NullUUID{UUID: p.EmployeeID, Valid: true}
	switch permission.ScopeOf(p.Role, key) {
	case permission.ScopeAll:
		return ScopeFilter{All: true}, nil
	case permission.ScopeSubdivision:
		ids, err := s.q.HeadedSubdivisionIDs(ctx, head)
		if err != nil {
			return ScopeFilter{}, err
		}
		return ScopeFilter{SubdivisionIDs: ids}, nil
	case permission.ScopeDivision:
		ids, err := s.q.HeadedDivisionIDs(ctx, head)
		if err != nil {
			return ScopeFilter{}, err
		}
		return ScopeFilter{DivisionIDs: ids}, nil
	default:
		return ScopeFilter{}, httpx.ErrForbidden()
	}
}

func (s *Service) issue(userID, employeeID, sessionID uuid.UUID, role, refresh string, now time.Time) (Tokens, error) {
	access, err := s.signer.Sign(userID, employeeID, sessionID, role, now)
	if err != nil {
		return Tokens{}, err
	}
	csrf, err := NewOpaqueToken()
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, Refresh: refresh, CSRF: csrf}, nil
}

func (s *Service) auditBestEffort(ctx context.Context, e audit.Entry) {
	if err := audit.Write(ctx, s.q, e); err != nil && !errors.Is(err, context.Canceled) {
		slog.ErrorContext(ctx, "gagal menulis audit", "action", e.Action, "error", err.Error())
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
