// Package org mengelola divisi dan subdiv.
package org

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: store.New(pool)} }

// Division adalah objek baku respons divisi.
type Division struct {
	ID               string  `json:"id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	HeadEmployeeID   *string `json:"head_employee_id"`
	HeadName         *string `json:"head_name"`
	SubdivisionCount int64   `json:"subdivision_count"`
	EmployeeCount    int64   `json:"employee_count"`
	CreatedAt        *string `json:"created_at"`
}

// Subdivision adalah objek baku respons subdiv.
type Subdivision struct {
	ID             string  `json:"id"`
	DivisionID     string  `json:"division_id"`
	DivisionName   string  `json:"division_name"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	HeadEmployeeID *string `json:"head_employee_id"`
	HeadName       *string `json:"head_name"`
	EmployeeCount  int64   `json:"employee_count"`
	CreatedAt      *string `json:"created_at"`
}

// DivisionInput juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya.
type DivisionInput struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	HeadEmployeeID *string `json:"head_employee_id"`
}

type SubdivisionInput struct {
	DivisionID     string  `json:"division_id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	HeadEmployeeID *string `json:"head_employee_id"`
}

func validateCodeName(code, name string, maxCode int, fields map[string]string) {
	switch {
	case code == "":
		fields["code"] = "Kode wajib diisi."
	case len(code) > maxCode:
		fields["code"] = "Kode terlalu panjang."
	case !codePattern.MatchString(code):
		fields["code"] = "Kode hanya huruf, angka, dan tanda minus."
	}
	switch {
	case name == "":
		fields["name"] = "Nama wajib diisi."
	case len(name) > 80:
		fields["name"] = "Nama maksimal 80 karakter."
	}
}

func (in *DivisionInput) normalize() (uuid.NullUUID, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	fields := map[string]string{}
	validateCodeName(in.Code, in.Name, 10, fields)
	head, err := pgconv.ParseOptionalUUID(in.HeadEmployeeID)
	if err != nil {
		fields["head_employee_id"] = "Kepala divisi tidak valid."
	}
	if len(fields) > 0 {
		return head, httpx.Validation(fields)
	}
	return head, nil
}

func (in *SubdivisionInput) normalize() (uuid.UUID, uuid.NullUUID, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	fields := map[string]string{}
	validateCodeName(in.Code, in.Name, 12, fields)
	divisionID, err := uuid.Parse(in.DivisionID)
	if err != nil {
		fields["division_id"] = "Pilih divisi."
	}
	head, err := pgconv.ParseOptionalUUID(in.HeadEmployeeID)
	if err != nil {
		fields["head_employee_id"] = "Kepala subdiv tidak valid."
	}
	if len(fields) > 0 {
		return divisionID, head, httpx.Validation(fields)
	}
	return divisionID, head, nil
}

func errHasDependents(message string) error {
	return httpx.NewError(http.StatusConflict, "CONFLICT_HAS_DEPENDENTS", message)
}

func mapUnique(err error, label string) error {
	if _, ok := pgconv.UniqueConstraint(err); ok {
		return httpx.Duplicate("code", "Kode "+label+" sudah dipakai.")
	}
	return err
}

func (s *Service) ListDivisions(ctx context.Context) ([]Division, error) {
	rows, err := s.q.ListDivisions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Division, 0, len(rows))
	for _, r := range rows {
		out = append(out, Division{
			ID: r.ID.String(), Code: r.Code, Name: r.Name,
			HeadEmployeeID: pgconv.UUIDPtr(r.HeadEmployeeID), HeadName: pgconv.TextPtr(r.HeadName),
			SubdivisionCount: r.SubdivisionCount, EmployeeCount: r.EmployeeCount,
			CreatedAt: pgconv.TimeString(r.CreatedAt),
		})
	}
	return out, nil
}

func (s *Service) divisionDetail(ctx context.Context, id uuid.UUID) (Division, error) {
	r, err := s.q.GetDivisionDetail(ctx, id)
	if pgconv.IsNoRows(err) {
		return Division{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Division{}, err
	}
	return Division{
		ID: r.ID.String(), Code: r.Code, Name: r.Name,
		HeadEmployeeID: pgconv.UUIDPtr(r.HeadEmployeeID), HeadName: pgconv.TextPtr(r.HeadName),
		SubdivisionCount: r.SubdivisionCount, EmployeeCount: r.EmployeeCount,
		CreatedAt: pgconv.TimeString(r.CreatedAt),
	}, nil
}

func (s *Service) CreateDivision(ctx context.Context, actor audit.Entry, in DivisionInput) (Division, error) {
	head, err := in.normalize()
	if err != nil {
		return Division{}, err
	}
	if head.Valid {
		return Division{}, httpx.Validation(map[string]string{"head_employee_id": "Divisi baru belum memiliki Pejuang."})
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.CreateDivision(ctx, store.CreateDivisionParams{Code: in.Code, Name: in.Name, HeadEmployeeID: head})
		if err != nil {
			return mapUnique(err, "divisi")
		}
		id = row.ID
		return writeOrgAudit(ctx, q, actor, "division", id, "created")
	})
	if err != nil {
		return Division{}, err
	}
	return s.divisionDetail(ctx, id)
}

func (s *Service) UpdateDivision(ctx context.Context, actor audit.Entry, id uuid.UUID, decode func(*DivisionInput) error) (Division, error) {
	current, err := s.q.GetDivision(ctx, id)
	if pgconv.IsNoRows(err) {
		return Division{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Division{}, err
	}
	in := DivisionInput{Code: current.Code, Name: current.Name, HeadEmployeeID: pgconv.UUIDPtr(current.HeadEmployeeID)}
	if err := decode(&in); err != nil {
		return Division{}, err
	}
	head, err := in.normalize()
	if err != nil {
		return Division{}, err
	}
	if head.Valid {
		ok, err := s.q.EmployeeBelongsToDivision(ctx, store.EmployeeBelongsToDivisionParams{ID: head.UUID, DivisionID: id})
		if err != nil {
			return Division{}, err
		}
		if !ok {
			return Division{}, httpx.Validation(map[string]string{"head_employee_id": "Kepala divisi harus Pejuang di divisi ini."})
		}
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.UpdateDivision(ctx, store.UpdateDivisionParams{ID: id, Code: in.Code, Name: in.Name, HeadEmployeeID: head}); err != nil {
			return mapUnique(err, "divisi")
		}
		if err := syncHeadRoles(ctx, q, actor, current.HeadEmployeeID, head); err != nil {
			return err
		}
		return writeOrgAudit(ctx, q, actor, "division", id, "updated")
	})
	if err != nil {
		return Division{}, err
	}
	return s.divisionDetail(ctx, id)
}

func (s *Service) DeleteDivision(ctx context.Context, actor audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		count, err := q.CountSubdivisionsInDivision(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return errHasDependents("Divisi masih memiliki subdiv. Pindahkan atau hapus subdiv terlebih dahulu.")
		}
		affected, err := q.DeleteDivision(ctx, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return writeOrgAudit(ctx, q, actor, "division", id, "deleted")
	})
}

func (s *Service) ListSubdivisions(ctx context.Context, divisionID uuid.NullUUID) ([]Subdivision, error) {
	rows, err := s.q.ListSubdivisions(ctx, divisionID)
	if err != nil {
		return nil, err
	}
	out := make([]Subdivision, 0, len(rows))
	for _, r := range rows {
		out = append(out, Subdivision{
			ID: r.ID.String(), DivisionID: r.DivisionID.String(), DivisionName: r.DivisionName,
			Code: r.Code, Name: r.Name,
			HeadEmployeeID: pgconv.UUIDPtr(r.HeadEmployeeID), HeadName: pgconv.TextPtr(r.HeadName),
			EmployeeCount: r.EmployeeCount, CreatedAt: pgconv.TimeString(r.CreatedAt),
		})
	}
	return out, nil
}

func (s *Service) subdivisionDetail(ctx context.Context, id uuid.UUID) (Subdivision, error) {
	r, err := s.q.GetSubdivisionDetail(ctx, id)
	if pgconv.IsNoRows(err) {
		return Subdivision{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Subdivision{}, err
	}
	return Subdivision{
		ID: r.ID.String(), DivisionID: r.DivisionID.String(), DivisionName: r.DivisionName,
		Code: r.Code, Name: r.Name,
		HeadEmployeeID: pgconv.UUIDPtr(r.HeadEmployeeID), HeadName: pgconv.TextPtr(r.HeadName),
		EmployeeCount: r.EmployeeCount, CreatedAt: pgconv.TimeString(r.CreatedAt),
	}, nil
}

func (s *Service) assertDivisionExists(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.GetDivision(ctx, id)
	if pgconv.IsNoRows(err) {
		return httpx.Validation(map[string]string{"division_id": "Divisi tidak ditemukan."})
	}
	return err
}

func (s *Service) CreateSubdivision(ctx context.Context, actor audit.Entry, in SubdivisionInput) (Subdivision, error) {
	divisionID, head, err := in.normalize()
	if err != nil {
		return Subdivision{}, err
	}
	if head.Valid {
		return Subdivision{}, httpx.Validation(map[string]string{"head_employee_id": "Subdiv baru belum memiliki Pejuang."})
	}
	if err := s.assertDivisionExists(ctx, divisionID); err != nil {
		return Subdivision{}, err
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.CreateSubdivision(ctx, store.CreateSubdivisionParams{DivisionID: divisionID, Code: in.Code, Name: in.Name, HeadEmployeeID: head})
		if err != nil {
			return mapUnique(err, "subdiv")
		}
		id = row.ID
		return writeOrgAudit(ctx, q, actor, "subdivision", id, "created")
	})
	if err != nil {
		return Subdivision{}, err
	}
	return s.subdivisionDetail(ctx, id)
}

func (s *Service) UpdateSubdivision(ctx context.Context, actor audit.Entry, id uuid.UUID, decode func(*SubdivisionInput) error) (Subdivision, error) {
	current, err := s.q.GetSubdivision(ctx, id)
	if pgconv.IsNoRows(err) {
		return Subdivision{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Subdivision{}, err
	}
	in := SubdivisionInput{
		DivisionID: current.DivisionID.String(), Code: current.Code, Name: current.Name,
		HeadEmployeeID: pgconv.UUIDPtr(current.HeadEmployeeID),
	}
	if err := decode(&in); err != nil {
		return Subdivision{}, err
	}
	divisionID, head, err := in.normalize()
	if err != nil {
		return Subdivision{}, err
	}
	if err := s.assertDivisionExists(ctx, divisionID); err != nil {
		return Subdivision{}, err
	}
	if head.Valid {
		ok, err := s.q.EmployeeBelongsToSubdivision(ctx, store.EmployeeBelongsToSubdivisionParams{ID: head.UUID, SubdivisionID: id})
		if err != nil {
			return Subdivision{}, err
		}
		if !ok {
			return Subdivision{}, httpx.Validation(map[string]string{"head_employee_id": "Kepala subdiv harus Pejuang di subdiv ini."})
		}
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.UpdateSubdivision(ctx, store.UpdateSubdivisionParams{
			ID: id, DivisionID: divisionID, Code: in.Code, Name: in.Name, HeadEmployeeID: head,
		}); err != nil {
			return mapUnique(err, "subdiv")
		}
		if err := syncHeadRoles(ctx, q, actor, current.HeadEmployeeID, head); err != nil {
			return err
		}
		return writeOrgAudit(ctx, q, actor, "subdivision", id, "updated")
	})
	if err != nil {
		return Subdivision{}, err
	}
	return s.subdivisionDetail(ctx, id)
}

func (s *Service) DeleteSubdivision(ctx context.Context, actor audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		// Pejuang nonaktif/terhapus tetap dihitung: riwayatnya masih merujuk subdiv ini.
		count, err := q.CountEmployeesAnyStateInSubdivision(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return errHasDependents("Subdiv masih memiliki data Pejuang. Pindahkan Pejuang terlebih dahulu.")
		}
		affected, err := q.DeleteSubdivision(ctx, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return writeOrgAudit(ctx, q, actor, "subdivision", id, "deleted")
	})
}

func writeOrgAudit(ctx context.Context, q *store.Queries, actor audit.Entry, entity string, id uuid.UUID, change string) error {
	actor.Action = audit.OrgUpdated
	actor.Entity = entity
	actor.EntityID = id
	actor.Detail = map[string]any{"change": change, "at": time.Now().UTC().Format(time.RFC3339)}
	return audit.Write(ctx, q, actor)
}

// syncHeadRoles menyesuaikan role kepala lama dan baru (D49): kepala divisi menjadi kadiv, kepala
// subdiv menjadi subdiv, dan yang tidak lagi memimpin unit mana pun kembali menjadi pejuang. Role
// sdm dan superadmin tidak pernah diubah. Sesi berjalan memakai role baru setelah token diperbarui.
func syncHeadRoles(ctx context.Context, q *store.Queries, actor audit.Entry, before, after uuid.NullUUID) error {
	if before == after {
		return nil
	}
	for _, id := range []uuid.NullUUID{before, after} {
		if !id.Valid {
			continue
		}
		st, err := q.GetHeadRoleState(ctx, id.UUID)
		if pgconv.IsNoRows(err) {
			continue // Pejuang tanpa akun
		}
		if err != nil {
			return err
		}
		if st.Role == store.UserRoleSdm || st.Role == store.UserRoleSuperadmin {
			continue
		}
		want := store.UserRolePejuang
		switch {
		case st.HeadsDivision:
			want = store.UserRoleKadiv
		case st.HeadsSubdivision:
			want = store.UserRoleSubdiv
		}
		if want == st.Role {
			continue
		}
		if err := q.SetUserRole(ctx, store.SetUserRoleParams{ID: st.UserID, Role: want}); err != nil {
			return err
		}
		e := actor
		e.Action, e.Entity, e.EntityID = audit.UserRoleChanged, "user", st.UserID
		e.Detail = map[string]any{"from": st.Role, "to": want, "reason": "head_assignment"}
		if err := audit.Write(ctx, q, e); err != nil {
			return err
		}
	}
	return nil
}
