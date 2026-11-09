package attendance

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

type remoteFaceStatus struct {
	ExpiresAt string `json:"expires_at"`
	Reason    string `json:"reason"`
}

func (s *Service) remoteFacePermission(ctx context.Context, p auth.Principal, ip *netip.Addr, employeeID uuid.UUID, method, reason string) (*remoteFaceStatus, error) {
	if employeeID == p.EmployeeID {
		return nil, httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Izin sendiri harus dikelola peninjau lain.")
	}
	reason = strings.TrimSpace(reason)
	if method == http.MethodPut && (reason == "" || len([]rune(reason)) > 500) {
		return nil, httpx.Validation(map[string]string{"reason": "Alasan wajib diisi, maksimal 500 karakter."})
	}
	var result *remoteFaceStatus
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if _, err := q.LockEmployeeForDevice(ctx, employeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, employeeID); err != nil {
			return err
		}
		switch method {
		case http.MethodGet:
			row, err := q.GetRemoteFacePermission(ctx, store.GetRemoteFacePermissionParams{EmployeeID: employeeID, Now: pgconv.Time(s.opt.Now())})
			if pgconv.IsNoRows(err) {
				return nil
			}
			if err != nil {
				return err
			}
			result = &remoteFaceStatus{ExpiresAt: row.ExpiresAt.Time.UTC().Format(time.RFC3339), Reason: row.Reason}
			return nil
		case http.MethodPut:
			if err := q.SetRemoteFacePermission(ctx, store.SetRemoteFacePermissionParams{EmployeeID: employeeID, ApprovedBy: p.UserID, Reason: reason, ExpiresAt: pgconv.Time(s.opt.Now().Add(24 * time.Hour))}); err != nil {
				return err
			}
			return audit.Write(ctx, q, audit.Entry{UserID: audit.Actor(p.UserID), Action: audit.FaceRemotePermissionGranted, Entity: "employee", EntityID: employeeID, IP: ip, Detail: map[string]any{"reason": reason, "duration_hours": 24}})
		case http.MethodDelete:
			n, err := q.DeleteRemoteFacePermission(ctx, employeeID)
			if err != nil {
				return err
			}
			if n == 0 {
				return httpx.ErrNotFound()
			}
			return audit.Write(ctx, q, audit.Entry{UserID: audit.Actor(p.UserID), Action: audit.FaceRemotePermissionRevoked, Entity: "employee", EntityID: employeeID, IP: ip})
		}
		return httpx.ErrNotFound()
	})
	return result, err
}

func (h *Handler) RemoteFacePermission(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if r.Method == http.MethodPut {
		if err := httpx.Decode(r, &input); err != nil {
			return err
		}
	}
	p, _ := auth.PrincipalFrom(r.Context())
	status, err := h.svc.remoteFacePermission(r.Context(), p, audit.ClientIP(r), id, r.Method, input.Reason)
	if err != nil {
		return err
	}
	if r.Method == http.MethodGet {
		return httpx.Data(w, http.StatusOK, status)
	}
	return httpx.NoContent(w)
}
