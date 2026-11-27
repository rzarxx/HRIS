package payroll

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

func actor(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func periodID(r *http.Request) (uuid.UUID, error) { return httpx.PathID(r) }

func (h *Handler) ListPeriods(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.ListPeriods(r.Context(), principalUser(r))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) CreatePeriod(w http.ResponseWriter, r *http.Request) error {
	var in PeriodInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreatePeriod(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) Period(w http.ResponseWriter, r *http.Request) error {
	id, err := periodID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.PeriodDetail(r.Context(), principalUser(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) DeletePeriod(w http.ResponseWriter, r *http.Request) error {
	id, err := periodID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeletePeriod(r.Context(), actor(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

// transition menjalankan aksi status lalu mengembalikan detail periode terbaru.
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, fn func(audit.Entry, uuid.UUID) error) error {
	id, err := periodID(r)
	if err != nil {
		return err
	}
	if err := fn(actor(r), id); err != nil {
		return err
	}
	out, err := h.svc.PeriodDetail(r.Context(), principalUser(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) error {
	return h.transition(w, r, func(e audit.Entry, id uuid.UUID) error {
		_, err := h.svc.Generate(r.Context(), e, id)
		return err
	})
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) error {
	return h.transition(w, r, func(e audit.Entry, id uuid.UUID) error { return h.svc.Submit(r.Context(), e, id) })
}

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) error {
	var in RejectInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	return h.transition(w, r, func(e audit.Entry, id uuid.UUID) error { return h.svc.Reject(r.Context(), e, id, in) })
}

func (h *Handler) Finalize(w http.ResponseWriter, r *http.Request) error {
	return h.transition(w, r, func(e audit.Entry, id uuid.UUID) error { return h.svc.Finalize(r.Context(), e, id) })
}

func (h *Handler) MarkPaid(w http.ResponseWriter, r *http.Request) error {
	return h.transition(w, r, func(e audit.Entry, id uuid.UUID) error { return h.svc.MarkPaid(r.Context(), e, id) })
}

func employeePathIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	id, err := periodID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	employeeID, err := uuid.Parse(chi.URLParam(r, "employee_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, httpx.ErrNotFound()
	}
	return id, employeeID, nil
}

func (h *Handler) EmployeePayroll(w http.ResponseWriter, r *http.Request) error {
	id, employeeID, err := employeePathIDs(r)
	if err != nil {
		return err
	}
	out, err := h.svc.EmployeePayroll(r.Context(), principalUser(r), id, employeeID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) SetInputs(w http.ResponseWriter, r *http.Request) error {
	id, employeeID, err := employeePathIDs(r)
	if err != nil {
		return err
	}
	var in InputsInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.SetInputs(r.Context(), actor(r), id, employeeID, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
