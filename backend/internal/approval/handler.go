package approval

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Pending(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Pending(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) error { return h.decide(w, r, true) }

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) error { return h.decide(w, r, false) }

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, approve bool) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in Decision
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &in); err != nil {
			return err
		}
	}
	p, _ := auth.PrincipalFrom(r.Context())
	var out Item
	if approve {
		out, err = h.svc.Approve(r.Context(), p, audit.ClientIP(r), id, in)
	} else {
		out, err = h.svc.Reject(r.Context(), p, audit.ClientIP(r), id, in)
	}
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Flow(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Flow(r.Context(), chi.URLParam(r, "request_type"))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) SetFlow(w http.ResponseWriter, r *http.Request) error {
	var in FlowInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.SetFlow(r.Context(), p, audit.ClientIP(r), chi.URLParam(r, "request_type"), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
