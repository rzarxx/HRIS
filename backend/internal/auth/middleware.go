package auth

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

const (
	CookieAccess  = "hris_at"
	CookieRefresh = "hris_rt"
	CookieCSRF    = "hris_csrf"
	HeaderCSRF    = "X-CSRF-Token"
)

// Principal adalah identitas hasil verifikasi token.
type Principal struct {
	UserID     uuid.UUID
	EmployeeID uuid.UUID
	SessionID  uuid.UUID
	Role       string
	// Syarat yang belum dipenuhi; fitur selain akun sendiri ditolak sampai dipenuhi.
	MustChangePassword bool
	MFASetupRequired   bool
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom mengembalikan identitas; handler di balik Authenticate selalu memilikinya.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// SessionState adalah keadaan sesi terkini dari database, bukan dari klaim JWT.
type SessionState struct {
	Active             bool
	MustChangePassword bool
	MFASetupRequired   bool
}

// SessionChecker memeriksa sesi; sesi dicabut = token langsung mati.
type SessionChecker func(ctx context.Context, sessionID uuid.UUID) (SessionState, error)

// Authenticate memverifikasi cookie hris_at lalu mengecek sesi dan CSRF (urutan F2).
func Authenticate(signer *TokenSigner, active SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieAccess)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			claims, err := signer.Verify(cookie.Value)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			p, ok := principalFromClaims(claims)
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			state, err := active(r.Context(), p.SessionID)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			if !state.Active {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			p.MustChangePassword = state.MustChangePassword
			p.MFASetupRequired = state.MFASetupRequired
			if !CSRFValid(r) {
				httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "CSRF_INVALID", "Token keamanan tidak valid, muat ulang halaman."))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

func principalFromClaims(c *Claims) (Principal, bool) {
	userID, err1 := uuid.Parse(c.Subject)
	employeeID, err2 := uuid.Parse(c.EID)
	sessionID, err3 := uuid.Parse(c.SID)
	if err1 != nil || err2 != nil || err3 != nil {
		return Principal{}, false
	}
	return Principal{UserID: userID, EmployeeID: employeeID, SessionID: sessionID, Role: c.Role}, true
}

// CSRFValid menerapkan double-submit cookie pada semua method selain GET.
func CSRFValid(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	cookie, err := r.Cookie(CookieCSRF)
	if err != nil || cookie.Value == "" {
		return false
	}
	header := r.Header.Get(HeaderCSRF)
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) == 1
}

// Require menolak bila role tidak memiliki SEMUA permission yang diminta.
func Require(keys ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			// Sandi sementara dan MFA wajib lebih dulu dipenuhi (F3, F4).
			if p.MustChangePassword {
				httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "Ganti kata sandi terlebih dahulu."))
				return
			}
			if p.MFASetupRequired {
				httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "MFA_SETUP_REQUIRED", "Aktifkan verifikasi dua langkah terlebih dahulu."))
				return
			}
			for _, key := range keys {
				if !permission.Allows(p.Role, key) {
					httpx.WriteError(w, r, httpx.ErrForbidden())
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
