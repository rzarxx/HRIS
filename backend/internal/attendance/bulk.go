package attendance

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Alasan Pejuang dilewati pada penetapan massal.
const (
	SkipSelf      = "self"
	SkipProtected = "protected"
	SkipOverlap   = "overlap"
)

// BulkArrangementInput menetapkan izin yang sama untuk semua Pejuang aktif di satu divisi atau subdiv.
type BulkArrangementInput struct {
	ArrangementInput
	DivisionID    *string `json:"division_id"`
	SubdivisionID *string `json:"subdivision_id"`
}

// BulkSkip adalah Pejuang yang tidak mendapat izin, beserta alasannya.
type BulkSkip struct {
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Reason       string `json:"reason"`
}

type BulkArrangementResult struct {
	Created int        `json:"created"`
	Skipped []BulkSkip `json:"skipped"`
}

func (in BulkArrangementInput) unit() (uuid.NullUUID, uuid.NullUUID, error) {
	var division, subdivision uuid.NullUUID
	parse := func(raw *string, field, message string) (uuid.NullUUID, error) {
		id, err := uuid.Parse(*raw)
		if err != nil {
			return uuid.NullUUID{}, httpx.Validation(map[string]string{field: message})
		}
		return uuid.NullUUID{UUID: id, Valid: true}, nil
	}
	var err error
	switch {
	case (in.DivisionID == nil) == (in.SubdivisionID == nil):
		err = httpx.Validation(map[string]string{"division_id": "Pilih salah satu: divisi atau subdiv."})
	case in.DivisionID != nil:
		division, err = parse(in.DivisionID, "division_id", "Divisi tidak valid.")
	default:
		subdivision, err = parse(in.SubdivisionID, "subdivision_id", "Subdiv tidak valid.")
	}
	return division, subdivision, err
}

// CreateArrangementsBulk menyimpan izin per Pejuang agar pengecualian dan auditnya jelas (MASTER.md §5.4.2).
// Pejuang yang tidak boleh diberi izin dilewati, bukan menggagalkan semuanya: diri sendiri, super admin
// bila yang menetapkan SDM (D34), dan yang izinnya bertabrakan.
func (s *Service) CreateArrangementsBulk(ctx context.Context, p auth.Principal, ip *netip.Addr, in BulkArrangementInput) (BulkArrangementResult, error) {
	division, subdivision, err := in.unit()
	if err != nil {
		return BulkArrangementResult{}, err
	}
	parsed, err := in.parse()
	if err != nil {
		return BulkArrangementResult{}, err
	}
	out := BulkArrangementResult{Skipped: []BulkSkip{}}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		targets, err := q.ListArrangementTargets(ctx, store.ListArrangementTargetsParams{DivisionID: division, SubdivisionID: subdivision})
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			field := "division_id"
			if subdivision.Valid {
				field = "subdivision_id"
			}
			return httpx.Validation(map[string]string{field: "Tidak ada Pejuang aktif di unit ini."})
		}
		skip := func(t store.ListArrangementTargetsRow, reason string) {
			out.Skipped = append(out.Skipped, BulkSkip{EmployeeID: t.ID.String(), EmployeeName: t.FullName, Reason: reason})
		}
		// Urutan id yang sama dengan query mencegah deadlock antar penetapan massal.
		for _, t := range targets {
			if t.ID == p.EmployeeID {
				skip(t, SkipSelf)
				continue
			}
			if _, err := q.LockEmployeeForDevice(ctx, t.ID); err != nil {
				return err
			}
			if err := auth.AssertEmployeeNotProtected(ctx, q, p, t.ID); err != nil {
				var apiErr *httpx.Error
				if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
					skip(t, SkipProtected)
					continue
				}
				return err
			}
			existing, err := q.ListWorkArrangements(ctx, t.ID)
			if err != nil {
				return err
			}
			overlap := false
			for _, other := range existing {
				if parsed.rule.Overlaps(arrangementRule(other)) {
					overlap = true
					break
				}
			}
			if overlap {
				skip(t, SkipOverlap)
				continue
			}
			row, err := q.InsertWorkArrangement(ctx, store.InsertWorkArrangementParams{
				EmployeeID: t.ID, Mode: in.Mode, FlexibleHours: in.FlexibleHours, StartsOn: parsed.startsOn,
				EndsOn: parsed.endsOn, Weekdays: parsed.weekdays, Reason: parsed.reason, CreatedBy: p.UserID,
			})
			if err != nil {
				return err
			}
			entry := arrangementAudit(p, ip, row, "work_arrangement_added")
			entry.Detail["bulk"] = true
			if err := audit.Write(ctx, q, entry); err != nil {
				return err
			}
			out.Created++
		}
		return nil
	})
	if err != nil {
		return BulkArrangementResult{}, err
	}
	return out, nil
}

func (h *Handler) CreateArrangementsBulk(w http.ResponseWriter, r *http.Request) error {
	var in BulkArrangementInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.CreateArrangementsBulk(r.Context(), p, audit.ClientIP(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
