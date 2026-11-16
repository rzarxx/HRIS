package org

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Branch adalah cabang yayasan dengan struktur divisi, subdiv, dan atasan sendiri (D53).
type Branch struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	DivisionCount int64   `json:"division_count"`
	EmployeeCount int64   `json:"employee_count"`
	CreatedAt     *string `json:"created_at"`
}

type BranchInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (in *BranchInput) normalize() error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	fields := map[string]string{}
	validateCodeName(in.Code, in.Name, 20, fields)
	if _, ok := fields["code"]; !ok && len(in.Code) < 2 {
		fields["code"] = "Kode minimal 2 karakter."
	}
	if len(fields) > 0 {
		return httpx.Validation(fields)
	}
	return nil
}

func mapBranchUnique(err error) error {
	constraint, ok := pgconv.UniqueConstraint(err)
	if !ok {
		return err
	}
	if strings.Contains(constraint, "name") {
		return httpx.Duplicate("name", "Nama cabang sudah dipakai.")
	}
	return httpx.Duplicate("code", "Kode cabang sudah dipakai.")
}

func (s *Service) ListBranches(ctx context.Context) ([]Branch, error) {
	rows, err := s.q.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Branch, 0, len(rows))
	for _, r := range rows {
		out = append(out, Branch{
			ID: r.ID.String(), Code: r.Code, Name: r.Name,
			DivisionCount: r.DivisionCount, EmployeeCount: r.EmployeeCount, CreatedAt: pgconv.TimeString(r.CreatedAt),
		})
	}
	return out, nil
}

func (s *Service) branchDetail(ctx context.Context, id uuid.UUID) (Branch, error) {
	rows, err := s.ListBranches(ctx)
	if err != nil {
		return Branch{}, err
	}
	for _, b := range rows {
		if b.ID == id.String() {
			return b, nil
		}
	}
	return Branch{}, httpx.ErrNotFound()
}

func (s *Service) CreateBranch(ctx context.Context, actor audit.Entry, in BranchInput) (Branch, error) {
	if err := in.normalize(); err != nil {
		return Branch{}, err
	}
	var id uuid.UUID
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.CreateBranch(ctx, store.CreateBranchParams{Code: in.Code, Name: in.Name})
		if err != nil {
			return mapBranchUnique(err)
		}
		id = row.ID
		return writeOrgAudit(ctx, q, actor, "branch", id, "created")
	})
	if err != nil {
		return Branch{}, err
	}
	return s.branchDetail(ctx, id)
}

func (s *Service) UpdateBranch(ctx context.Context, actor audit.Entry, id uuid.UUID, decode func(*BranchInput) error) (Branch, error) {
	current, err := s.q.GetBranch(ctx, id)
	if pgconv.IsNoRows(err) {
		return Branch{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Branch{}, err
	}
	in := BranchInput{Code: current.Code, Name: current.Name}
	if err := decode(&in); err != nil {
		return Branch{}, err
	}
	if err := in.normalize(); err != nil {
		return Branch{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.UpdateBranch(ctx, store.UpdateBranchParams{ID: id, Code: in.Code, Name: in.Name}); err != nil {
			return mapBranchUnique(err)
		}
		return writeOrgAudit(ctx, q, actor, "branch", id, "updated")
	})
	if err != nil {
		return Branch{}, err
	}
	return s.branchDetail(ctx, id)
}

func (s *Service) DeleteBranch(ctx context.Context, actor audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		count, err := q.CountDivisionsInBranch(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return errHasDependents("Cabang masih memiliki divisi. Pindahkan atau hapus divisi terlebih dahulu.")
		}
		affected, err := q.DeleteBranch(ctx, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return writeOrgAudit(ctx, q, actor, "branch", id, "deleted")
	})
}

func (s *Service) assertBranchExists(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.GetBranch(ctx, id)
	if pgconv.IsNoRows(err) {
		return httpx.Validation(map[string]string{"branch_id": "Cabang tidak ditemukan."})
	}
	return err
}

func (h *Handler) ListBranches(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.ListBranches(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateBranch(w http.ResponseWriter, r *http.Request) error {
	var in BranchInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateBranch(r.Context(), actorFrom(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) UpdateBranch(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.UpdateBranch(r.Context(), actorFrom(r), id, func(in *BranchInput) error {
		return httpx.Decode(r, in)
	})
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) DeleteBranch(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteBranch(r.Context(), actorFrom(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
