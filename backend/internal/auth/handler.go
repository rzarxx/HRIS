package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

const (
	loginAttemptLimit    = 5
	loginAttemptWindow   = 15 * time.Minute
	activateLimit        = 10
	activationCheckLimit = 60
)

// Handler melayani /v1/auth/* dan /v1/me.
type Handler struct {
	svc          *Service
	secure       bool
	cookieDomain string
	limiter      Limiter
}

// NewHandler: secure=false hanya untuk pengembangan lokal tanpa HTTPS.
func NewHandler(svc *Service, secure bool, cookieDomain string, limiter Limiter) *Handler {
	return &Handler{svc: svc, secure: secure, cookieDomain: cookieDomain, limiter: limiter}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (req *loginRequest) problems() map[string]string {
	req.Email = strings.TrimSpace(req.Email)
	fields := map[string]string{}
	if req.Email == "" || !strings.Contains(req.Email, "@") || len(req.Email) > 254 {
		fields["email"] = "Email tidak valid."
	}
	if req.Password == "" {
		fields["password"] = "Kata sandi wajib diisi."
	} else if len(req.Password) > 256 {
		fields["password"] = "Kata sandi terlalu panjang."
	}
	return fields
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if fields := req.problems(); len(fields) > 0 {
		return httpx.Validation(fields)
	}

	// Batas per akun + IP (MASTER.md §5.7), melengkapi lockout per akun.
	limitKey := "login:" + strings.ToLower(req.Email)
	if err := h.checkLimit(w, r, limitKey, loginAttemptLimit, loginAttemptWindow); err != nil {
		return err
	}
	userID, tokens, err := h.svc.Login(r.Context(), req.Email, req.Password, audit.ClientIP(r), r.UserAgent(), time.Now())
	// Sandi benar (termasuk yang lanjut ke langkah MFA) tidak dihitung sebagai percobaan gagal.
	var apiErr *httpx.Error
	if err == nil || (errors.As(err, &apiErr) && apiErr.Code == "AUTH_MFA_REQUIRED") {
		h.resetLimit(r, limitKey)
	}
	if err != nil {
		return err
	}
	return h.finishLogin(w, r, userID, tokens)
}

type mobileLoginRequest struct {
	loginRequest
	MFACode string       `json:"mfa_code"`
	Device  MobileDevice `json:"device"`
}

// MobileLogin menerbitkan token di body, bukan cookie. Batas percobaannya sama dengan login web.
func (h *Handler) MobileLogin(w http.ResponseWriter, r *http.Request) error {
	var req mobileLoginRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	fields := req.problems()
	if len(req.MFACode) > 64 {
		fields["mfa_code"] = "Kode terlalu panjang."
	}
	if len(fields) > 0 {
		return httpx.Validation(fields)
	}
	limitKey := "login:" + strings.ToLower(req.Email)
	if err := h.checkLimit(w, r, limitKey, loginAttemptLimit, loginAttemptWindow); err != nil {
		return err
	}
	tokens, err := h.svc.LoginMobile(r.Context(), req.Email, req.Password, strings.TrimSpace(req.MFACode), req.Device, audit.ClientIP(r), r.UserAgent(), time.Now())
	if err != nil {
		return err
	}
	h.resetLimit(r, limitKey)
	w.Header().Set("Cache-Control", "no-store")
	return httpx.Data(w, http.StatusOK, tokens)
}

type mobileRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) MobileRefresh(w http.ResponseWriter, r *http.Request) error {
	var req mobileRefreshRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	tokens, err := h.svc.RefreshMobile(r.Context(), req.RefreshToken, audit.ClientIP(r), time.Now())
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.Data(w, http.StatusOK, tokens)
}

func (h *Handler) finishLogin(w http.ResponseWriter, r *http.Request, userID uuid.UUID, tokens Tokens) error {
	h.setCookies(w, tokens)
	me, err := h.svc.Me(r.Context(), userID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, me)
}

func limitKeyWithIP(r *http.Request, key string) string {
	if addr := audit.ClientIP(r); addr != nil {
		return key + "|" + addr.String()
	}
	return key + "|"
}

func (h *Handler) resetLimit(r *http.Request, key string) {
	h.limiter.Reset(limitKeyWithIP(r, key))
}

func (h *Handler) checkLimit(w http.ResponseWriter, r *http.Request, key string, limit int, window time.Duration) error {
	ok, wait := h.limiter.Allow(limitKeyWithIP(r, key), limit, window, time.Now())
	if ok {
		return nil
	}
	return RateLimited(w, wait)
}

// RateLimited menyetel Retry-After dan mengembalikan error RATE_LIMITED.
func RateLimited(w http.ResponseWriter, wait time.Duration) error {
	seconds := int(wait.Seconds()) + 1
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	return &httpx.Error{
		Status:  http.StatusTooManyRequests,
		Code:    "RATE_LIMITED",
		Message: "Terlalu banyak percobaan. Coba lagi nanti.",
		Details: map[string]any{"retry_after_seconds": seconds},
	}
}

type mfaVerifyRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

func (h *Handler) MFAVerify(w http.ResponseWriter, r *http.Request) error {
	var req mfaVerifyRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 64 {
		return httpx.Validation(map[string]string{"code": "Kode wajib diisi."})
	}
	if err := h.checkLimit(w, r, "mfa:"+HashToken(req.MFAToken), loginAttemptLimit, MFATokenTTL); err != nil {
		return err
	}
	userID, tokens, err := h.svc.VerifyMFA(r.Context(), req.MFAToken, req.Code, audit.ClientIP(r), r.UserAgent(), time.Now())
	if err != nil {
		return err
	}
	return h.finishLogin(w, r, userID, tokens)
}

type activateRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) error {
	var req activateRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.checkLimit(w, r, "activate", activateLimit, loginAttemptWindow); err != nil {
		return err
	}
	if err := h.svc.Activate(r.Context(), req.Token, req.Password, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

type activationCheckRequest struct {
	Token string `json:"token"`
}

type activationCheckResponse struct {
	ExpiresAt string `json:"expires_at"`
}

// CheckActivation tidak mengubah apa pun. Batasnya lebih longgar dari aktivasi karena dipanggil setiap halaman dibuka.
func (h *Handler) CheckActivation(w http.ResponseWriter, r *http.Request) error {
	var req activationCheckRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.checkLimit(w, r, "activate-check", activationCheckLimit, loginAttemptWindow); err != nil {
		return err
	}
	expires, err := h.svc.CheckActivation(r.Context(), req.Token)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, activationCheckResponse{ExpiresAt: expires.UTC().Format(time.RFC3339)})
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	var req passwordChangeRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.checkLimit(w, r, "password:"+p.UserID.String(), loginAttemptLimit, loginAttemptWindow); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(r.Context(), p, req.CurrentPassword, req.NewPassword, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	rows, err := h.svc.ListSessions(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound()
	}
	return id, nil
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	id, err := pathUUID(r, "session_id")
	if err != nil {
		return err
	}
	if err := h.svc.RevokeOwnSession(r.Context(), p, id, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) MFASetup(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	out, err := h.svc.SetupMFA(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

type mfaEnableRequest struct {
	Code string `json:"code"`
}

func (h *Handler) MFAEnable(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	var req mfaEnableRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.checkLimit(w, r, "mfa-enable:"+p.UserID.String(), loginAttemptLimit, MFATokenTTL); err != nil {
		return err
	}
	codes, err := h.svc.EnableMFA(r.Context(), p, req.Code, audit.ClientIP(r), time.Now())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (h *Handler) adminTarget(r *http.Request) (Principal, uuid.UUID, error) {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return p, uuid.Nil, httpx.ErrUnauthenticated()
	}
	id, err := pathUUID(r, "user_id")
	return p, id, err
}

func (h *Handler) AdminResetPassword(w http.ResponseWriter, r *http.Request) error {
	p, id, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	out, err := h.svc.ResetPassword(r.Context(), p, id, audit.ClientIP(r), time.Now())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) AdminLock(w http.ResponseWriter, r *http.Request) error {
	p, id, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := h.svc.LockUser(r.Context(), p, id, audit.ClientIP(r), time.Now()); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) AdminUnlock(w http.ResponseWriter, r *http.Request) error {
	p, id, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := h.svc.UnlockUser(r.Context(), p, id, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) AdminRevokeSessions(w http.ResponseWriter, r *http.Request) error {
	p, id, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := h.svc.RevokeUserSessions(r.Context(), p, id, audit.ClientIP(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	if !CSRFValid(r) {
		return httpx.NewError(http.StatusForbidden, "CSRF_INVALID", "Token keamanan tidak valid, muat ulang halaman.")
	}
	refresh := ""
	if c, err := r.Cookie(CookieRefresh); err == nil {
		refresh = c.Value
	}
	tokens, err := h.svc.Refresh(r.Context(), refresh, audit.ClientIP(r), time.Now())
	if err != nil {
		h.clearCookies(w)
		return err
	}
	h.setCookies(w, tokens)
	return httpx.NoContent(w)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	if err := h.svc.Logout(r.Context(), p, audit.ClientIP(r)); err != nil {
		return err
	}
	h.clearCookies(w)
	return httpx.NoContent(w)
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return httpx.ErrUnauthenticated()
	}
	me, err := h.svc.Me(r.Context(), p.UserID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, me)
}

func (h *Handler) setCookies(w http.ResponseWriter, t Tokens) {
	// Token tetap host-only di host API, jadi tidak ikut terkirim ke subdomain lain.
	h.setCookie(w, CookieAccess, t.Access, "/v1", "", int(AccessTTL.Seconds()), true)
	h.setCookie(w, CookieRefresh, t.Refresh, "/v1/auth/web", "", int(webAbsoluteTTL.Seconds()), true)
	// hris_csrf sengaja bisa dibaca JavaScript web admin di subdomain lain untuk dikirim
	// ulang lewat header, sehingga memakai domain induk.
	h.setCookie(w, CookieCSRF, t.CSRF, "/", h.cookieDomain, int(webAbsoluteTTL.Seconds()), false)
}

func (h *Handler) clearCookies(w http.ResponseWriter) {
	h.setCookie(w, CookieAccess, "", "/v1", "", -1, true)
	h.setCookie(w, CookieRefresh, "", "/v1/auth/web", "", -1, true)
	h.setCookie(w, CookieCSRF, "", "/", h.cookieDomain, -1, false)
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value, path, domain string, maxAge int, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   domain,
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})
}
