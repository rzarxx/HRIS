package storageadmin

import (
	"net/http"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Settings(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Save(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.SyncBatch(r.Context(), audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)})
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
