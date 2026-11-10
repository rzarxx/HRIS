// Package location mengelola lokasi kantor dan penugasan lokasi presensi per Pejuang.
package location

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	minRadiusM      = 10
	maxRadiusM      = 5000
	maxPolygonPoint = 100
)

// OfficeLocation adalah objek baku respons.
type OfficeLocation struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Latitude      float64            `json:"latitude"`
	Longitude     float64            `json:"longitude"`
	RadiusM       int32              `json:"radius_m"`
	Polygon       []attendance.Point `json:"polygon"`
	IsActive      bool               `json:"is_active"`
	EmployeeCount int64              `json:"employee_count"`
}

// Input juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya.
type Input struct {
	Name      string             `json:"name"`
	Latitude  *float64           `json:"latitude"`
	Longitude *float64           `json:"longitude"`
	RadiusM   *int32             `json:"radius_m"`
	Polygon   []attendance.Point `json:"polygon"`
	IsActive  *bool              `json:"is_active"`
}

type validated struct {
	Name      string
	Latitude  decimal.Decimal
	Longitude decimal.Decimal
	RadiusM   int32
	Polygon   []byte
	IsActive  bool
}

func (in Input) validate() (validated, error) {
	fields := map[string]string{}
	v := validated{Name: strings.TrimSpace(in.Name), IsActive: in.IsActive == nil || *in.IsActive}
	switch {
	case v.Name == "":
		fields["name"] = "Nama lokasi wajib diisi."
	case len(v.Name) > 120:
		fields["name"] = "Nama maksimal 120 karakter."
	}
	if in.Latitude == nil || math.IsNaN(*in.Latitude) || *in.Latitude < -90 || *in.Latitude > 90 {
		fields["latitude"] = "Lintang harus antara -90 dan 90."
	} else {
		v.Latitude = decimal.NewFromFloat(*in.Latitude).Round(6)
	}
	if in.Longitude == nil || math.IsNaN(*in.Longitude) || *in.Longitude < -180 || *in.Longitude > 180 {
		fields["longitude"] = "Bujur harus antara -180 dan 180."
	} else {
		v.Longitude = decimal.NewFromFloat(*in.Longitude).Round(6)
	}
	if in.RadiusM == nil || *in.RadiusM < minRadiusM || *in.RadiusM > maxRadiusM {
		fields["radius_m"] = "Radius harus antara 10 dan 5000 meter."
	} else {
		v.RadiusM = *in.RadiusM
	}
	if in.Polygon != nil {
		if len(in.Polygon) < 3 || len(in.Polygon) > maxPolygonPoint {
			fields["polygon"] = "Poligon harus 3 sampai 100 titik."
		}
		for _, p := range in.Polygon {
			if !p.Valid() {
				fields["polygon"] = "Ada titik poligon yang tidak valid."
				break
			}
		}
		if fields["polygon"] == "" {
			raw, err := json.Marshal(in.Polygon)
			if err != nil {
				return v, err
			}
			v.Polygon = raw
		}
	}
	if len(fields) > 0 {
		return v, httpx.Validation(fields)
	}
	return v, nil
}

// Service memuat aturan lokasi.
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: store.New(pool)} }

func decodePolygon(raw []byte) ([]attendance.Point, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var polygon []attendance.Point
	if err := json.Unmarshal(raw, &polygon); err != nil {
		return nil, err
	}
	return polygon, nil
}

func toResponse(id uuid.UUID, name string, lat, lon decimal.Decimal, radius int32, polygon []byte, active bool, count int64) (OfficeLocation, error) {
	points, err := decodePolygon(polygon)
	if err != nil {
		return OfficeLocation{}, err
	}
	return OfficeLocation{
		ID: id.String(), Name: name, Latitude: lat.InexactFloat64(), Longitude: lon.InexactFloat64(),
		RadiusM: radius, Polygon: points, IsActive: active, EmployeeCount: count,
	}, nil
}

func (s *Service) List(ctx context.Context) ([]OfficeLocation, error) {
	rows, err := s.q.ListOfficeLocations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OfficeLocation, 0, len(rows))
	for _, r := range rows {
		item, err := toResponse(r.ID, r.Name, r.Latitude, r.Longitude, r.RadiusM, r.Polygon, r.IsActive, r.EmployeeCount)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, id uuid.UUID) (OfficeLocation, error) {
	r, err := s.q.GetOfficeLocation(ctx, id)
	if pgconv.IsNoRows(err) {
		return OfficeLocation{}, httpx.ErrNotFound()
	}
	if err != nil {
		return OfficeLocation{}, err
	}
	return toResponse(r.ID, r.Name, r.Latitude, r.Longitude, r.RadiusM, r.Polygon, r.IsActive, r.EmployeeCount)
}

func changeAudit(ctx context.Context, q *store.Queries, entry audit.Entry, entity string, id uuid.UUID, change string) error {
	entry.Action, entry.Entity, entry.EntityID = audit.SettingsChanged, entity, id
	entry.Detail = map[string]any{"change": change}
	return audit.Write(ctx, q, entry)
}

func (s *Service) Create(ctx context.Context, entry audit.Entry, in Input) (OfficeLocation, error) {
	v, err := in.validate()
	if err != nil {
		return OfficeLocation{}, err
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		id, err = q.CreateOfficeLocation(ctx, store.CreateOfficeLocationParams{
			Name: v.Name, Latitude: v.Latitude, Longitude: v.Longitude, RadiusM: v.RadiusM, Polygon: v.Polygon, IsActive: v.IsActive,
		})
		if err != nil {
			return err
		}
		return changeAudit(ctx, q, entry, "office_location", id, "created")
	})
	if err != nil {
		return OfficeLocation{}, err
	}
	return s.get(ctx, id)
}

// Update memuat data lama lebih dulu, lalu menimpa dengan field yang dikirim klien.
func (s *Service) Update(ctx context.Context, entry audit.Entry, id uuid.UUID, decode func(*Input) error) (OfficeLocation, error) {
	current, err := s.get(ctx, id)
	if err != nil {
		return OfficeLocation{}, err
	}
	in := Input{
		Name: current.Name, Latitude: &current.Latitude, Longitude: &current.Longitude,
		RadiusM: &current.RadiusM, Polygon: current.Polygon, IsActive: &current.IsActive,
	}
	if err := decode(&in); err != nil {
		return OfficeLocation{}, err
	}
	v, err := in.validate()
	if err != nil {
		return OfficeLocation{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.UpdateOfficeLocation(ctx, store.UpdateOfficeLocationParams{
			ID: id, Name: v.Name, Latitude: v.Latitude, Longitude: v.Longitude, RadiusM: v.RadiusM, Polygon: v.Polygon, IsActive: v.IsActive,
		}); err != nil {
			return err
		}
		return changeAudit(ctx, q, entry, "office_location", id, "updated")
	})
	if err != nil {
		return OfficeLocation{}, err
	}
	return s.get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.DeleteOfficeLocation(ctx, id)
		if pgconv.IsForeignKeyViolation(err) {
			return httpx.NewError(http.StatusConflict, "CONFLICT_HAS_DEPENDENTS", "Lokasi masih ditugaskan ke Pejuang. Nonaktifkan atau pindahkan penugasannya dulu.")
		}
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		return changeAudit(ctx, q, entry, "office_location", id, "deleted")
	})
}

func (s *Service) assertEmployee(ctx context.Context, id uuid.UUID) error {
	exists, err := s.q.EmployeeExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return httpx.ErrNotFound()
	}
	return nil
}

// ForEmployee mengembalikan lokasi presensi yang diizinkan untuk seorang Pejuang.
func (s *Service) ForEmployee(ctx context.Context, employeeID uuid.UUID) ([]OfficeLocation, error) {
	if err := s.assertEmployee(ctx, employeeID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListEmployeeLocations(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	out := make([]OfficeLocation, 0, len(rows))
	for _, r := range rows {
		item, err := toResponse(r.ID, r.Name, r.Latitude, r.Longitude, r.RadiusM, r.Polygon, r.IsActive, r.EmployeeCount)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// SetForEmployee menggantikan seluruh daftar lokasi seorang Pejuang.
func (s *Service) SetForEmployee(ctx context.Context, p auth.Principal, entry audit.Entry, employeeID uuid.UUID, rawIDs []string) ([]OfficeLocation, error) {
	if err := s.assertEmployee(ctx, employeeID); err != nil {
		return nil, err
	}
	if err := auth.AssertEmployeeNotProtected(ctx, s.q, p, employeeID); err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, httpx.Validation(map[string]string{"location_ids": "Ada lokasi yang tidak valid."})
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.ClearEmployeeLocations(ctx, employeeID); err != nil {
			return err
		}
		for _, id := range ids {
			if err := q.AddEmployeeLocation(ctx, store.AddEmployeeLocationParams{EmployeeID: employeeID, LocationID: id}); err != nil {
				if pgconv.IsForeignKeyViolation(err) {
					return httpx.Validation(map[string]string{"location_ids": "Ada lokasi yang tidak ditemukan."})
				}
				return err
			}
		}
		entry.Action, entry.Entity, entry.EntityID = audit.EmployeeUpdated, "employee", employeeID
		entry.Detail = map[string]any{"change": "locations", "count": len(ids)}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return nil, err
	}
	return s.ForEmployee(ctx, employeeID)
}

// Handler melayani /v1/admin/office-locations dan lokasi per Pejuang.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.Create(r.Context(), actor(r), in)
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
	out, err := h.svc.Update(r.Context(), actor(r), id, func(in *Input) error { return httpx.Decode(r, in) })
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), actor(r), id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ForEmployee(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	rows, err := h.svc.ForEmployee(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

type employeeLocationsInput struct {
	LocationIDs *[]string `json:"location_ids"`
}

func (h *Handler) SetForEmployee(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in employeeLocationsInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.LocationIDs == nil {
		return httpx.Validation(map[string]string{"location_ids": "Daftar lokasi wajib dikirim."})
	}
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := h.svc.SetForEmployee(r.Context(), p, actor(r), id, *in.LocationIDs)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}
