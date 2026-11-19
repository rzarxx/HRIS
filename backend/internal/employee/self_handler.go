package employee

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

// MeFunc membentuk objek GET /v1/me (auth.Service.Me), dipakai sebagai jawaban setelah profil diubah.
type MeFunc func(ctx context.Context, userID, deviceID uuid.UUID) (auth.Me, error)

// WithMe memasang pembentuk objek Me untuk jawaban endpoint profil diri.
func (h *Handler) WithMe(fn MeFunc) *Handler {
	h.me = fn
	return h
}

func (h *Handler) respondMe(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	me, err := h.me(r.Context(), p.UserID, p.DeviceID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, me)
}

// UpdateMe menangani PATCH /v1/me.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	p, entry := actor(r)
	err := h.svc.UpdateOwnContact(r.Context(), p, entry, func(in *ContactInput) error {
		return httpx.Decode(r, in)
	})
	if err != nil {
		return err
	}
	return h.respondMe(w, r, p)
}

// MyChildren menangani GET /v1/me/children.
func (h *Handler) MyChildren(w http.ResponseWriter, r *http.Request) error {
	p, _ := actor(r)
	out, err := h.svc.OwnFamily(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// SetMyPhoto menangani PUT /v1/me/photo; body adalah isi berkas JPEG atau PNG.
func (h *Handler) SetMyPhoto(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxPhotoUploadBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errImportTooLarge()
		}
		return err
	}
	if len(body) > MaxPhotoUploadBytes {
		return errImportTooLarge()
	}
	p, entry := actor(r)
	if err := h.svc.SetOwnPhoto(r.Context(), p, entry, body, time.Now()); err != nil {
		return err
	}
	return h.respondMe(w, r, p)
}

// DeleteMyPhoto menangani DELETE /v1/me/photo.
func (h *Handler) DeleteMyPhoto(w http.ResponseWriter, r *http.Request) error {
	p, entry := actor(r)
	if err := h.svc.DeleteOwnPhoto(r.Context(), p, entry); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

// Photo menangani GET /v1/employees/{id}/photo.
func (h *Handler) Photo(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := actor(r)
	data, err := h.svc.Photo(r.Context(), p, id)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	return err
}
