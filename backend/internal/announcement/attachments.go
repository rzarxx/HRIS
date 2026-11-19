package announcement

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	// MaxAttachmentBytes sama dengan batas body rute unggah (MASTER.md §5.7).
	MaxAttachmentBytes = 5 << 20
	// MaxAttachments per pengumuman (D54).
	MaxAttachments = 5
	maxFileName    = 120
)

// FileStore menyimpan berkas lampiran terenkripsi; DiskStore presensi (SELFIE_DIR) memenuhinya.
type FileStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// WithFiles memasang penyimpanan lampiran. Tanpa ini, unggah lampiran menjawab STORAGE_UNAVAILABLE.
func (s *Service) WithFiles(files FileStore) *Service {
	s.files = files
	return s
}

// Attachment adalah metadata satu lampiran (MASTER.md §6, D54).
type Attachment struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int    `json:"size_bytes"`
}

func errStorageUnavailable() error {
	return httpx.NewError(http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Penyimpanan berkas belum tersedia. Silakan hubungi pengelola server.")
}

func errAttachmentTooLarge() error {
	return httpx.NewError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran berkas paling besar 5 MB.")
}

// detectAttachment mengenali jenis berkas dari magic bytes, bukan dari Content-Type kiriman klien.
func detectAttachment(data []byte) (contentType, ext string, ok bool) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "application/pdf", ".pdf", true
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", ".jpg", true
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		return "image/png", ".png", true
	}
	return "", "", false
}

// cleanFileName membuang folder dan karakter kontrol, lalu memastikan ekstensi sesuai jenis berkas.
func cleanFileName(raw, ext string) string {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '"' {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "" {
		name = "lampiran"
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		base = "lampiran"
	}
	if runes := []rune(base); len(runes) > maxFileName-len(ext) {
		base = string(runes[:maxFileName-len(ext)])
	}
	return base + ext
}

// attachmentsFor mengambil lampiran untuk banyak pengumuman sekaligus.
func (s *Service) attachmentsFor(ctx context.Context, q *store.Queries, ids []uuid.UUID) (map[uuid.UUID][]Attachment, error) {
	out := make(map[uuid.UUID][]Attachment, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListAnnouncementAttachments(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AnnouncementID] = append(out[r.AnnouncementID], Attachment{
			ID: r.ID.String(), FileName: r.FileName, ContentType: r.ContentType, SizeBytes: int(r.SizeBytes),
		})
	}
	return out, nil
}

// AddAttachment menyimpan satu lampiran baru pada pengumuman.
func (s *Service) AddAttachment(ctx context.Context, entry audit.Entry, id uuid.UUID, name string, data []byte) (Announcement, error) {
	if s.files == nil {
		return Announcement{}, errStorageUnavailable()
	}
	if _, err := s.q.GetAnnouncement(ctx, id); pgconv.IsNoRows(err) {
		return Announcement{}, httpx.ErrNotFound()
	} else if err != nil {
		return Announcement{}, err
	}
	data, contentType, ext, err := s.prepareAttachment(ctx, data)
	if err != nil {
		return Announcement{}, err
	}
	fileName := cleanFileName(name, ext)
	packed, suffix, err := packAttachment(data)
	if err != nil {
		return Announcement{}, err
	}
	key := fmt.Sprintf("announcements/%s/%s%s%s", id, uuid.New(), ext, suffix)
	if err := s.files.Put(ctx, key, packed); err != nil {
		return Announcement{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		// Batas dihitung di dalam transaksi agar dua unggahan bersamaan tidak melewatinya.
		if err := q.LockAnnouncement(ctx, id); err != nil {
			return err
		}
		n, err := q.CountAnnouncementAttachments(ctx, id)
		if err != nil {
			return err
		}
		if n >= MaxAttachments {
			return httpx.Validation(map[string]string{"file": fmt.Sprintf("Lampiran paling banyak %d berkas per pengumuman.", MaxAttachments)})
		}
		if _, err := q.CreateAnnouncementAttachment(ctx, store.CreateAnnouncementAttachmentParams{
			AnnouncementID: id, FileName: fileName, ContentType: contentType, SizeBytes: int32(len(data)),
			StorageKey: key, CreatedBy: entry.UserID,
		}); err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.AnnouncementUpdated, "announcement", id
		entry.Detail = map[string]any{"attachment_added": fileName}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		s.deleteFile(ctx, key)
		return Announcement{}, err
	}
	return s.Get(ctx, id)
}

// DeleteAttachment menghapus satu lampiran beserta berkasnya.
func (s *Service) DeleteAttachment(ctx context.Context, entry audit.Entry, id, attachmentID uuid.UUID) error {
	var key string
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		key, err = q.DeleteAnnouncementAttachment(ctx, store.DeleteAnnouncementAttachmentParams{ID: attachmentID, AnnouncementID: id})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		entry.Action, entry.Entity, entry.EntityID = audit.AnnouncementUpdated, "announcement", id
		entry.Detail = map[string]any{"attachment_removed": attachmentID.String()}
		return audit.Write(ctx, q, entry)
	})
	if err != nil {
		return err
	}
	s.deleteFile(ctx, key)
	return nil
}

// AttachmentFile mengembalikan isi lampiran. Pemegang announcement:manage boleh mengunduh lampiran
// pengumuman apa pun; selain itu hanya pengumuman yang sedang tayang bagi pemanggil, dan di luar itu
// dijawab 404 agar keberadaan pengumuman tidak bocor.
func (s *Service) AttachmentFile(ctx context.Context, p auth.Principal, id, attachmentID uuid.UUID) (store.AnnouncementAttachment, []byte, error) {
	if s.files == nil {
		return store.AnnouncementAttachment{}, nil, errStorageUnavailable()
	}
	if !permission.Allows(p.Role, permission.AnnouncementManage) {
		if _, err := s.activeFor(ctx, s.q, p.EmployeeID, id, s.now()); err != nil {
			return store.AnnouncementAttachment{}, nil, err
		}
	}
	a, err := s.q.GetAnnouncementAttachment(ctx, store.GetAnnouncementAttachmentParams{ID: attachmentID, AnnouncementID: id})
	if pgconv.IsNoRows(err) {
		return a, nil, httpx.ErrNotFound()
	}
	if err != nil {
		return a, nil, err
	}
	data, err := s.files.Get(ctx, a.StorageKey)
	if err != nil {
		return a, nil, err
	}
	data, err = unpackAttachment(a.StorageKey, data, int(a.SizeBytes))
	if err != nil {
		return a, nil, err
	}
	if !isProcessedAttachment(a.StorageKey) {
		if err := s.scan(ctx, data); err != nil {
			return a, nil, err
		}
	}
	return a, data, nil
}

func (s *Service) deleteFile(ctx context.Context, key string) {
	if key == "" || s.files == nil {
		return
	}
	if err := s.files.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "gagal menghapus berkas lampiran pengumuman", "err", err)
	}
}

// AddAttachment menangani POST /v1/admin/announcements/{id}/attachments.
func (h *Handler) AddAttachment(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	name := r.URL.Query().Get("name")
	if strings.TrimSpace(name) == "" {
		return httpx.Validation(map[string]string{"name": "Nama berkas wajib diisi."})
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxAttachmentBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errAttachmentTooLarge()
		}
		return err
	}
	if len(data) > MaxAttachmentBytes {
		return errAttachmentTooLarge()
	}
	if len(data) == 0 {
		return httpx.Validation(map[string]string{"file": "Berkas kosong."})
	}
	out, err := h.svc.AddAttachment(r.Context(), actor(r), id, name, data)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func attachmentID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "attachment_id"))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound()
	}
	return id, nil
}

// DeleteAttachment menangani DELETE /v1/admin/announcements/{id}/attachments/{attachment_id}.
func (h *Handler) DeleteAttachment(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	aid, err := attachmentID(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteAttachment(r.Context(), actor(r), id, aid); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

// Attachment menangani GET /v1/announcements/{id}/attachments/{attachment_id}.
func (h *Handler) Attachment(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	aid, err := attachmentID(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	a, data, err := h.svc.AttachmentFile(r.Context(), p, id, aid)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": a.FileName}))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	return err
}
