// Package announcement mengelola pengumuman pop-up dan banner.
package announcement

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: store.New(pool)} }

// Announcement adalah objek baku (MASTER.md §7.6).
type Announcement struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Body          string  `json:"body"`
	Type          string  `json:"type"`
	ImageUploadID *string `json:"image_upload_id"`
	DivisionID    *string `json:"division_id"`
	DivisionName  *string `json:"division_name"`
	StartsAt      *string `json:"starts_at"`
	EndsAt        *string `json:"ends_at"`
	IsActive      bool    `json:"is_active"`
	CreatedAt     *string `json:"created_at"`
}

// Input juga dipakai untuk PATCH: field yang tidak dikirim tetap nilai lamanya.
// image_upload_id diterima sesuai kontrak; modul unggah menyusul sehingga hanya boleh null.
type Input struct {
	Title         string  `json:"title"`
	Body          string  `json:"body"`
	Type          string  `json:"type"`
	ImageUploadID *string `json:"image_upload_id"`
	DivisionID    *string `json:"division_id"`
	StartsAt      string  `json:"starts_at"`
	EndsAt        *string `json:"ends_at"`
	IsActive      *bool   `json:"is_active"`
}

type validated struct {
	Title      string
	Body       string
	Type       store.AnnType
	DivisionID uuid.NullUUID
	StartsAt   pgtype.Timestamptz
	EndsAt     pgtype.Timestamptz
	IsActive   bool
}

// cleanText membuang karakter kontrol; isi pengumuman selalu teks murni (MASTER.md T11).
// Klien menampilkannya sebagai teks, tidak pernah sebagai HTML.
func cleanText(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s))
}

func (in Input) validate(ctx context.Context, q *store.Queries) (validated, error) {
	fields := map[string]string{}
	v := validated{Title: cleanText(in.Title), Body: cleanText(in.Body), IsActive: in.IsActive == nil || *in.IsActive}
	switch {
	case v.Title == "":
		fields["title"] = "Judul wajib diisi."
	case len(v.Title) > 120:
		fields["title"] = "Judul maksimal 120 karakter."
	}
	switch {
	case v.Body == "":
		fields["body"] = "Isi wajib diisi."
	case len(v.Body) > 2000:
		fields["body"] = "Isi maksimal 2000 karakter."
	}
	switch in.Type {
	case string(store.AnnTypePopup), string(store.AnnTypeBanner):
		v.Type = store.AnnType(in.Type)
	default:
		fields["type"] = "Pilih tipe pengumuman."
	}
	if in.ImageUploadID != nil && *in.ImageUploadID != "" {
		fields["image_upload_id"] = "Gambar pengumuman belum didukung."
	}
	division, err := pgconv.ParseOptionalUUID(in.DivisionID)
	if err != nil {
		fields["division_id"] = "Divisi tidak valid."
	} else if division.Valid {
		if _, err := q.GetDivision(ctx, division.UUID); pgconv.IsNoRows(err) {
			fields["division_id"] = "Divisi tidak ditemukan."
		} else if err != nil {
			return v, err
		}
	}
	v.DivisionID = division
	starts, err := time.Parse(time.RFC3339, in.StartsAt)
	if err != nil {
		fields["starts_at"] = "Waktu mulai tayang tidak valid."
	} else {
		v.StartsAt = pgconv.Time(starts)
	}
	if in.EndsAt != nil && *in.EndsAt != "" {
		ends, err := time.Parse(time.RFC3339, *in.EndsAt)
		switch {
		case err != nil:
			fields["ends_at"] = "Waktu berakhir tidak valid."
		case v.StartsAt.Valid && !ends.After(starts):
			fields["ends_at"] = "Waktu berakhir harus setelah mulai tayang."
		default:
			v.EndsAt = pgconv.Time(ends)
		}
	}
	if len(fields) > 0 {
		return v, httpx.Validation(fields)
	}
	return v, nil
}

func toResponse(r store.GetAnnouncementRow) Announcement {
	return Announcement{
		ID: r.ID.String(), Title: r.Title, Body: r.Body, Type: string(r.Type),
		ImageUploadID: pgconv.UUIDPtr(r.ImageUploadID),
		DivisionID:    pgconv.UUIDPtr(r.DivisionID), DivisionName: pgconv.TextPtr(r.DivisionName),
		StartsAt: pgconv.TimeString(r.StartsAt), EndsAt: pgconv.TimeString(r.EndsAt),
		IsActive: r.IsActive, CreatedAt: pgconv.TimeString(r.CreatedAt),
	}
}

func (s *Service) List(ctx context.Context, page httpx.Page) ([]Announcement, int64, error) {
	total, err := s.q.CountAnnouncements(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAnnouncements(ctx, store.ListAnnouncementsParams{PageLimit: page.Limit(), PageOffset: page.Offset()})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Announcement, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResponse(store.GetAnnouncementRow(r)))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Announcement, error) {
	r, err := s.q.GetAnnouncement(ctx, id)
	if pgconv.IsNoRows(err) {
		return Announcement{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Announcement{}, err
	}
	return toResponse(r), nil
}

func (s *Service) Create(ctx context.Context, entry audit.Entry, in Input) (Announcement, error) {
	v, err := in.validate(ctx, s.q)
	if err != nil {
		return Announcement{}, err
	}
	var id uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		id, err = q.CreateAnnouncement(ctx, store.CreateAnnouncementParams{
			Title: v.Title, Body: v.Body, Type: v.Type, DivisionID: v.DivisionID,
			StartsAt: v.StartsAt, EndsAt: v.EndsAt, IsActive: v.IsActive, CreatedBy: entry.UserID,
		})
		if err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.AnnouncementCreated, "announcement", id
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return Announcement{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, entry audit.Entry, id uuid.UUID, decode func(*Input) error) (Announcement, error) {
	current, err := s.q.GetAnnouncement(ctx, id)
	if pgconv.IsNoRows(err) {
		return Announcement{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Announcement{}, err
	}
	isActive := current.IsActive
	starts := pgconv.TimeString(current.StartsAt)
	in := Input{
		Title: current.Title, Body: current.Body, Type: string(current.Type),
		DivisionID: pgconv.UUIDPtr(current.DivisionID), StartsAt: *starts,
		EndsAt: pgconv.TimeString(current.EndsAt), IsActive: &isActive,
	}
	if err := decode(&in); err != nil {
		return Announcement{}, err
	}
	v, err := in.validate(ctx, s.q)
	if err != nil {
		return Announcement{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.UpdateAnnouncement(ctx, store.UpdateAnnouncementParams{
			ID: id, Title: v.Title, Body: v.Body, Type: v.Type, DivisionID: v.DivisionID,
			StartsAt: v.StartsAt, EndsAt: v.EndsAt, IsActive: v.IsActive,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		entry.Action, entry.Entity, entry.EntityID = audit.AnnouncementUpdated, "announcement", id
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return Announcement{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, entry audit.Entry, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.DeleteAnnouncement(ctx, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		entry.Action, entry.Entity, entry.EntityID = audit.AnnouncementDeleted, "announcement", id
		return audit.Write(ctx, q, entry)
	})
}

// Handler melayani /v1/admin/announcements.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	rows, total, err := h.svc.List(r.Context(), page)
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
	out, err := h.svc.Get(r.Context(), id)
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
