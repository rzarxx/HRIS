package mail

import (
	"net/http"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

// Handler melayani /v1/admin/settings/smtp (settings:manage, hanya super admin).
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Settings melayani GET /v1/admin/settings/smtp.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Settings(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// Save melayani PUT /v1/admin/settings/smtp. Email uji dikirim ke email akun yang sedang login.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	user, err := h.svc.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		return err
	}
	out, err := h.svc.Save(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}, user.Email.String, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// Disable melayani DELETE /v1/admin/settings/smtp.
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	if err := h.svc.Disable(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
