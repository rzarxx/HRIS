package attendance

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	ArrangementWFA    = "wfa"
	ArrangementHybrid = "hybrid"

	WorkModeOnsite = "onsite"
	WorkModeWFA    = "wfa"
	WorkModeField  = "field"

	maxReasonLen = 500
)

// WorkArrangement adalah objek baku izin kerja (MASTER.md §5.4.2).
type WorkArrangement struct {
	ID            string  `json:"id"`
	EmployeeID    string  `json:"employee_id"`
	Mode          string  `json:"mode"`
	FlexibleHours bool    `json:"flexible_hours"`
	StartsOn      string  `json:"starts_on"`
	EndsOn        *string `json:"ends_on"`
	Weekdays      []int16 `json:"weekdays"`
	Reason        string  `json:"reason"`
	CreatedAt     string  `json:"created_at"`
}

func presentArrangement(r store.EmployeeWorkArrangement) WorkArrangement {
	return WorkArrangement{
		ID: r.ID.String(), EmployeeID: r.EmployeeID.String(), Mode: r.Mode, FlexibleHours: r.FlexibleHours,
		StartsOn: r.StartsOn.Time.Format("2006-01-02"), EndsOn: pgconv.DateString(r.EndsOn),
		Weekdays: r.Weekdays, Reason: r.Reason, CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func arrangementRule(r store.EmployeeWorkArrangement) Arrangement {
	out := Arrangement{StartsOn: r.StartsOn.Time, Weekdays: r.Weekdays}
	if r.EndsOn.Valid {
		end := r.EndsOn.Time
		out.EndsOn = &end
	}
	return out
}

// ArrangementInput adalah isi penetapan izin kerja.
type ArrangementInput struct {
	Mode          string  `json:"mode"`
	FlexibleHours bool    `json:"flexible_hours"`
	StartsOn      string  `json:"starts_on"`
	EndsOn        *string `json:"ends_on"`
	Weekdays      []int16 `json:"weekdays"`
	Reason        string  `json:"reason"`
}

type parsedArrangement struct {
	startsOn pgtype.Date
	endsOn   pgtype.Date
	weekdays []int16
	reason   string
	rule     Arrangement
}

func (in ArrangementInput) parse() (parsedArrangement, error) {
	fields := map[string]string{}
	var p parsedArrangement
	if in.Mode != ArrangementWFA && in.Mode != ArrangementHybrid {
		fields["mode"] = "Pilih WFA atau hybrid."
	}
	starts, err := pgconv.ParseDate(in.StartsOn)
	if err != nil {
		fields["starts_on"] = "Tanggal mulai tidak valid."
	}
	p.startsOn = starts
	p.rule.StartsOn = starts.Time
	if in.EndsOn != nil {
		ends, err := pgconv.ParseDate(*in.EndsOn)
		switch {
		case err != nil:
			fields["ends_on"] = "Tanggal selesai tidak valid."
		case starts.Valid && ends.Time.Before(starts.Time):
			fields["ends_on"] = "Tanggal selesai tidak boleh sebelum tanggal mulai."
		}
		p.endsOn = ends
		end := ends.Time
		p.rule.EndsOn = &end
	}
	p.weekdays = []int16{}
	for _, d := range in.Weekdays {
		if d < 1 || d > 7 {
			fields["weekdays"] = "Hari harus bernilai 1 sampai 7."
			break
		}
		if !slices.Contains(p.weekdays, d) {
			p.weekdays = append(p.weekdays, d)
		}
	}
	slices.Sort(p.weekdays)
	p.rule.Weekdays = p.weekdays
	p.reason = strings.TrimSpace(in.Reason)
	if p.reason == "" {
		fields["reason"] = "Alasan wajib diisi."
	} else if len(p.reason) > maxReasonLen {
		fields["reason"] = "Alasan maksimal 500 karakter."
	}
	if len(fields) == 0 && !p.rule.Overlaps(p.rule) {
		fields["weekdays"] = "Tidak ada hari yang cocok dalam rentang tanggal ini."
	}
	if len(fields) > 0 {
		return p, httpx.Validation(fields)
	}
	return p, nil
}

// ListArrangements mengembalikan semua izin kerja seorang Pejuang.
func (s *Service) ListArrangements(ctx context.Context, employeeID uuid.UUID) ([]WorkArrangement, error) {
	exists, err := s.q.EmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, httpx.ErrNotFound()
	}
	rows, err := s.q.ListWorkArrangements(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkArrangement, 0, len(rows))
	for _, r := range rows {
		out = append(out, presentArrangement(r))
	}
	return out, nil
}

// CreateArrangement menetapkan izin kerja. Peninjau tidak boleh memberi izin untuk dirinya sendiri,
// karena izin ini melewati geofence. Izin yang tumpang tindih ditolak agar tiap tanggal hanya punya
// satu aturan.
func (s *Service) CreateArrangement(ctx context.Context, p auth.Principal, ip *netip.Addr, employeeID uuid.UUID, in ArrangementInput) (WorkArrangement, error) {
	if employeeID == p.EmployeeID {
		return WorkArrangement{}, httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Izin kerja sendiri harus diberikan peninjau lain.")
	}
	parsed, err := in.parse()
	if err != nil {
		return WorkArrangement{}, err
	}
	var row store.EmployeeWorkArrangement
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.LockEmployeeForDevice(ctx, employeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, employeeID); err != nil {
			return err
		}
		existing, err := q.ListWorkArrangements(ctx, employeeID)
		if err != nil {
			return err
		}
		for _, other := range existing {
			if parsed.rule.Overlaps(arrangementRule(other)) {
				return httpx.Duplicate("starts_on", "Tanggal ini bertabrakan dengan izin kerja lain. Hapus izin itu dulu.")
			}
		}
		row, err = q.InsertWorkArrangement(ctx, store.InsertWorkArrangementParams{
			EmployeeID: employeeID, Mode: in.Mode, FlexibleHours: in.FlexibleHours, StartsOn: parsed.startsOn,
			EndsOn: parsed.endsOn, Weekdays: parsed.weekdays, Reason: parsed.reason, CreatedBy: p.UserID,
		})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, arrangementAudit(p, ip, row, "work_arrangement_added"))
	})
	if err != nil {
		return WorkArrangement{}, err
	}
	return presentArrangement(row), nil
}

// DeleteArrangement mencabut izin kerja dan langsung berlaku, termasuk untuk presensi pulang hari itu.
// Riwayatnya tetap ada di audit, dan presensi yang sudah tercatat tidak berubah.
func (s *Service) DeleteArrangement(ctx context.Context, p auth.Principal, ip *netip.Addr, employeeID, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.LockEmployeeForDevice(ctx, employeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, employeeID); err != nil {
			return err
		}
		row, err := q.DeleteWorkArrangement(ctx, store.DeleteWorkArrangementParams{ID: id, EmployeeID: employeeID})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, arrangementAudit(p, ip, row, "work_arrangement_removed"))
	})
}

func arrangementAudit(p auth.Principal, ip *netip.Addr, r store.EmployeeWorkArrangement, change string) audit.Entry {
	return audit.Entry{
		UserID: audit.Actor(p.UserID), Action: audit.EmployeeWorkArrangementChanged, Entity: "employee", EntityID: r.EmployeeID, IP: ip,
		Detail: map[string]any{
			"change": change, "arrangement_id": r.ID, "mode": r.Mode, "flexible_hours": r.FlexibleHours,
			"starts_on": r.StartsOn.Time.Format("2006-01-02"), "ends_on": pgconv.DateString(r.EndsOn), "weekdays": r.Weekdays,
		},
	}
}

// placement adalah hasil penentuan tempat kerja satu upaya presensi.
type placement struct {
	workMode string
	location *store.ListEmployeeLocationsRow
	flexible bool
}

func (pl placement) locationID() uuid.NullUUID {
	if pl.location == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: pl.location.ID, Valid: true}
}

func (pl placement) locationRef() *LocationRef {
	if pl.location == nil {
		return nil
	}
	return &LocationRef{ID: pl.location.ID.String(), Name: pl.location.Name}
}

// place menentukan tempat kerja dari work_date, jadi klien tidak bisa memintanya (§5.4.2). WFA hanya
// melewati geofence; pemeriksaan lain sudah dijalankan verify. GPS tetap tersimpan sebagai bukti.
func (s *Service) place(ctx context.Context, a attempt, date time.Time, point Point) (placement, error) {
	rows, err := s.q.ListWorkArrangements(ctx, a.p.EmployeeID)
	if err != nil {
		return placement{}, err
	}
	var active *store.EmployeeWorkArrangement
	for i := range rows {
		if arrangementRule(rows[i]).ActiveOn(date) {
			active = &rows[i]
			break
		}
	}
	out := placement{workMode: WorkModeOnsite}
	if active != nil {
		out.flexible = active.FlexibleHours
		if active.Mode == ArrangementWFA {
			out.workMode = WorkModeWFA
			return out, nil
		}
	}
	location, ok, err := s.matchLocation(ctx, a.p.EmployeeID, point)
	if err != nil {
		return placement{}, err
	}
	if ok {
		out.location = &location
		return out, nil
	}
	if active != nil {
		out.workMode = WorkModeField
		return out, nil
	}
	return placement{}, s.reject(ctx, a, "geofence_out", httpx.NewError(http.StatusUnprocessableEntity, "GEOFENCE_OUT", "Anda berada di luar area kantor."))
}

func (h *Handler) ListArrangements(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	rows, err := h.svc.ListArrangements(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateArrangement(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in ArrangementInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.CreateArrangement(r.Context(), p, audit.ClientIP(r), id, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) DeleteArrangement(w http.ResponseWriter, r *http.Request) error {
	employeeID, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(chi.URLParam(r, "arrangement_id"))
	if err != nil {
		return httpx.ErrNotFound()
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if err := h.svc.DeleteArrangement(r.Context(), p, audit.ClientIP(r), employeeID, id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
