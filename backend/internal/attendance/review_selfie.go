package attendance

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"os"
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

func (s *Service) reviewSelfie(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, purpose string) ([]byte, error) {
	if purpose != PurposeClockIn && purpose != PurposeClockOut {
		return nil, httpx.Validation(map[string]string{"purpose": "Jenis selfie tidak valid."})
	}
	if s.opt.Selfies == nil {
		return nil, httpx.ErrNotFound()
	}
	var photo []byte
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		capture, err := q.LockReviewSelfie(ctx, store.LockReviewSelfieParams{AttendanceID: id, Purpose: purpose, Cutoff: pgtype.Timestamptz{Time: s.opt.Now().UTC().Add(-90 * 24 * time.Hour), Valid: true}})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		photo, err = s.opt.Selfies.Get(ctx, capture.StorageKey)
		if errors.Is(err, os.ErrNotExist) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{UserID: audit.Actor(p.UserID), Action: audit.FaceCaptureViewed, Entity: "face_capture", EntityID: capture.ID, IP: ip, Detail: map[string]any{"purpose": purpose}})
	})
	return photo, err
}

func (h *Handler) ReviewSelfie(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	photo, err := h.svc.reviewSelfie(r.Context(), p, audit.ClientIP(r), id, chi.URLParam(r, "purpose"))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = w.Write(photo)
	return err
}
