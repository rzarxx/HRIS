package org

import (
	"net/http"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actorFrom(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func (h *Handler) ListDivisions(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.ListDivisions(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateDivision(w http.ResponseWriter, r *http.Request) error {
	var in DivisionInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateDivision(r.Context(), actorFrom(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) UpdateDivision(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.UpdateDivision(r.Context(), actorFrom(r), id, func(in *DivisionInput) error {
		return httpx.Decode(r, in)
	})
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) DeleteDivision(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteDivision(r.Context(), actorFrom(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ListSubdivisions(w http.ResponseWriter, r *http.Request) error {
	divisionID, err := httpx.QueryUUID(r, "division_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.ListSubdivisions(r.Context(), divisionID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateSubdivision(w http.ResponseWriter, r *http.Request) error {
	var in SubdivisionInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateSubdivision(r.Context(), actorFrom(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) UpdateSubdivision(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.UpdateSubdivision(r.Context(), actorFrom(r), id, func(in *SubdivisionInput) error {
		return httpx.Decode(r, in)
	})
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) DeleteSubdivision(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteSubdivision(r.Context(), actorFrom(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
