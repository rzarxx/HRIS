package announcement

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Mine adalah pengumuman yang sedang tayang untuk Pejuang yang masuk, beserta status bacanya.
type Mine struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Body           string  `json:"body"`
	Type           string  `json:"type"`
	StartsAt       *string `json:"starts_at"`
	EndsAt         *string `json:"ends_at"`
	MinReadSeconds int     `json:"min_read_seconds"`
	OpenedAt       *string `json:"opened_at"`
	ReadAt         *string `json:"read_at"`
	// Sisa detik sebelum "Sudah baca" diterima, dihitung server agar jam HP tidak berpengaruh.
	ReadWaitSeconds int `json:"read_wait_seconds"`
}

// Reader adalah satu Pejuang sasaran beserta status bacanya, untuk rekap SDM.
type Reader struct {
	EmployeeID      string  `json:"employee_id"`
	FullName        string  `json:"full_name"`
	NIP             string  `json:"nip"`
	SubdivisionName string  `json:"subdivision_name"`
	OpenedAt        *string `json:"opened_at"`
	ReadAt          *string `json:"read_at"`
}

// waitSeconds menghitung sisa jeda baca; nol bila sudah dibaca atau jedanya sudah lewat.
func waitSeconds(minRead int16, opened, read pgtype.Timestamptz, now time.Time) int {
	switch {
	case read.Valid:
		return 0
	case !opened.Valid:
		return int(minRead)
	}
	left := time.Duration(minRead)*time.Second - now.Sub(opened.Time)
	if left <= 0 {
		return 0
	}
	return int(math.Ceil(left.Seconds()))
}

func toMine(r store.GetActiveAnnouncementForRow, now time.Time) Mine {
	return Mine{
		ID: r.ID.String(), Title: r.Title, Body: r.Body, Type: string(r.Type),
		StartsAt: pgconv.TimeString(r.StartsAt), EndsAt: pgconv.TimeString(r.EndsAt),
		MinReadSeconds: int(r.MinReadSeconds),
		OpenedAt:       pgconv.TimeString(r.OpenedAt), ReadAt: pgconv.TimeString(r.ReadAt),
		ReadWaitSeconds: waitSeconds(r.MinReadSeconds, r.OpenedAt, r.ReadAt, now),
	}
}

// Active mengembalikan pengumuman yang sedang tayang untuk Pejuang: belum dibaca lebih dulu.
func (s *Service) Active(ctx context.Context, employeeID uuid.UUID) ([]Mine, error) {
	now := s.now()
	rows, err := s.q.ListActiveAnnouncementsFor(ctx, store.ListActiveAnnouncementsForParams{EmployeeID: employeeID, Now: pgconv.Time(now)})
	if err != nil {
		return nil, err
	}
	out := make([]Mine, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMine(store.GetActiveAnnouncementForRow(r), now))
	}
	return out, nil
}

func (s *Service) activeFor(ctx context.Context, q *store.Queries, employeeID, id uuid.UUID, now time.Time) (store.GetActiveAnnouncementForRow, error) {
	r, err := q.GetActiveAnnouncementFor(ctx, store.GetActiveAnnouncementForParams{EmployeeID: employeeID, ID: id, Now: pgconv.Time(now)})
	if pgconv.IsNoRows(err) {
		// Pengumuman divisi lain, belum atau sudah tidak tayang: sama dengan tidak ada.
		return r, httpx.ErrNotFound()
	}
	return r, err
}

// Open mencatat waktu pertama isi dibuka. Pemanggilan berikutnya tidak mengubah waktu itu, sehingga
// timer baca tidak bisa diulang.
func (s *Service) Open(ctx context.Context, employeeID, id uuid.UUID) (Mine, error) {
	now := s.now()
	var out store.GetActiveAnnouncementForRow
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := s.activeFor(ctx, q, employeeID, id, now); err != nil {
			return err
		}
		if err := q.OpenAnnouncement(ctx, store.OpenAnnouncementParams{AnnouncementID: id, EmployeeID: employeeID, Now: pgconv.Time(now)}); err != nil {
			return err
		}
		var err error
		out, err = s.activeFor(ctx, q, employeeID, id, now)
		return err
	})
	if err != nil {
		return Mine{}, err
	}
	return toMine(out, now), nil
}

func readTooSoon(message string, wait int) error {
	return &httpx.Error{
		Status: http.StatusUnprocessableEntity, Code: "READ_TOO_SOON", Message: message,
		Details: map[string]any{"wait_seconds": wait},
	}
}

// Read menandai sudah dibaca bila jeda baca sejak dibuka sudah lewat (D27). Idempoten setelah dibaca.
func (s *Service) Read(ctx context.Context, employeeID, id uuid.UUID) (Mine, error) {
	now := s.now()
	var out store.GetActiveAnnouncementForRow
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		r, err := s.activeFor(ctx, q, employeeID, id, now)
		if err != nil {
			return err
		}
		if r.ReadAt.Valid {
			out = r
			return nil
		}
		if !r.OpenedAt.Valid {
			return readTooSoon("Buka isi pengumuman dulu sebelum menandai sudah dibaca.", int(r.MinReadSeconds))
		}
		if wait := waitSeconds(r.MinReadSeconds, r.OpenedAt, r.ReadAt, now); wait > 0 {
			return readTooSoon("Baca dulu isinya. Tombol aktif setelah hitung mundur selesai.", wait)
		}
		if err := q.MarkAnnouncementRead(ctx, store.MarkAnnouncementReadParams{AnnouncementID: id, EmployeeID: employeeID, Now: pgconv.Time(now)}); err != nil {
			return err
		}
		out, err = s.activeFor(ctx, q, employeeID, id, now)
		return err
	})
	if err != nil {
		return Mine{}, err
	}
	return toMine(out, now), nil
}

// Readers mengembalikan sasaran pengumuman dengan status baca: belum membuka, lalu sudah membuka,
// lalu sudah membaca.
func (s *Service) Readers(ctx context.Context, id uuid.UUID) ([]Reader, error) {
	a, err := s.q.GetAnnouncement(ctx, id)
	if pgconv.IsNoRows(err) {
		return nil, httpx.ErrNotFound()
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListAnnouncementReaders(ctx, store.ListAnnouncementReadersParams{AnnouncementID: id, DivisionID: a.DivisionID})
	if err != nil {
		return nil, err
	}
	out := make([]Reader, 0, len(rows))
	for _, r := range rows {
		out = append(out, Reader{
			EmployeeID: r.EmployeeID.String(), FullName: r.FullName, NIP: r.Nip, SubdivisionName: r.SubdivisionName,
			OpenedAt: pgconv.TimeString(r.OpenedAt), ReadAt: pgconv.TimeString(r.ReadAt),
		})
	}
	return out, nil
}

func employee(r *http.Request) uuid.UUID {
	p, _ := auth.PrincipalFrom(r.Context())
	return p.EmployeeID
}

// Active melayani GET /v1/announcements/active.
func (h *Handler) Active(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Active(r.Context(), employee(r))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// Open melayani POST /v1/announcements/{id}/open.
func (h *Handler) Open(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.Open(r.Context(), employee(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// Read melayani POST /v1/announcements/{id}/read.
func (h *Handler) Read(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.Read(r.Context(), employee(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// Readers melayani GET /v1/admin/announcements/{id}/reads.
func (h *Handler) Readers(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.Readers(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
