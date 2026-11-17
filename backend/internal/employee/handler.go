package employee

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(r *http.Request) (auth.Principal, audit.Entry) {
	p, _ := auth.PrincipalFrom(r.Context())
	return p, audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	branchID, err := httpx.QueryUUID(r, "branch_id")
	if err != nil {
		return err
	}
	divisionID, err := httpx.QueryUUID(r, "division_id")
	if err != nil {
		return err
	}
	subdivisionID, err := httpx.QueryUUID(r, "subdivision_id")
	if err != nil {
		return err
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != "inactive" {
		return httpx.Validation(map[string]string{"status": "Mohon pilih status dari daftar."})
	}
	p, _ := actor(r)
	rows, total, err := h.svc.List(r.Context(), p, ListParams{
		Q:             strings.TrimSpace(r.URL.Query().Get("q")),
		BranchID:      branchID,
		DivisionID:    divisionID,
		SubdivisionID: subdivisionID,
		Status:        status,
		Page:          page,
	})
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, page.Meta(total))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := actor(r)
	out, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, entry := actor(r)
	out, err := h.svc.Create(r.Context(), p, entry, in, time.Now())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, entry := actor(r)
	out, err := h.svc.Update(r.Context(), p, entry, id, func(in *Input) error {
		return httpx.Decode(r, in)
	})
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, entry := actor(r)
	out, err := h.svc.Deactivate(r.Context(), p, entry, id, time.Now())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// ImportTemplate mengirim template xlsx impor Pejuang berisi daftar cabang, divisi, dan subdiv terbaru.
func (h *Handler) ImportTemplate(w http.ResponseWriter, r *http.Request) error {
	body, err := h.svc.ImportTemplate(r.Context())
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-impor-pejuang.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(body)
	return err
}

// Import menerima isi berkas xlsx sebagai body. dry_run=true (bawaan) hanya memeriksa; dry_run=false menyimpan.
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) error {
	dryRun := r.URL.Query().Get("dry_run") != "false"
	body, err := io.ReadAll(io.LimitReader(r.Body, ImportMaxBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errImportTooLarge()
		}
		return err
	}
	if len(body) > ImportMaxBytes {
		return errImportTooLarge()
	}
	p, entry := actor(r)
	out, err := h.svc.Import(r.Context(), p, entry, body, dryRun, time.Now())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
