package attendance

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/payroll"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Aksi koreksi pulang otomatis (D69).
const (
	AdjustConfirm = "confirm"  // jam pulang otomatis (akhir shift) sudah sesuai
	AdjustSetTime = "set_time" // ganti jam pulang
	AdjustAbsent  = "absent"   // Pejuang dinyatakan tidak hadir
)

// AutoClockOutItem adalah satu presensi yang ditutup otomatis dan menunggu dicek SDM.
type AutoClockOutItem struct {
	ID              string  `json:"id"`
	EmployeeID      string  `json:"employee_id"`
	EmployeeName    string  `json:"employee_name"`
	Nip             string  `json:"nip"`
	SubdivisionName string  `json:"subdivision_name"`
	DivisionName    string  `json:"division_name"`
	WorkDate        string  `json:"work_date"`
	Status          string  `json:"status"`
	ClockInAt       *string `json:"clock_in_at"`
	ClockOutAt      *string `json:"clock_out_at"`
	LateMinutes     int32   `json:"late_minutes"`
	WorkMode        string  `json:"work_mode"`
	ShiftName       string  `json:"shift_name"`
}

// AutoClockOuts mengembalikan presensi pulang otomatis yang belum dicek, terbaru dulu.
func (s *Service) AutoClockOuts(ctx context.Context, page httpx.Page) ([]AutoClockOutItem, int64, error) {
	total, err := s.q.CountAutoClockOuts(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAutoClockOuts(ctx, store.ListAutoClockOutsParams{PageLimit: page.Limit(), PageOffset: page.Offset()})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AutoClockOutItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, AutoClockOutItem{
			ID: r.ID.String(), EmployeeID: r.EmployeeID.String(), EmployeeName: r.EmployeeName, Nip: r.Nip,
			SubdivisionName: r.SubdivisionName, DivisionName: r.DivisionName, WorkDate: r.WorkDate.Time.Format(time.DateOnly),
			Status: string(r.Status), ClockInAt: pgconv.TimeString(r.ClockInAt), ClockOutAt: pgconv.TimeString(r.ClockOutAt),
			LateMinutes: r.LateMinutes, WorkMode: r.WorkMode, ShiftName: r.ShiftName,
		})
	}
	return out, total, nil
}

// AdjustInput adalah keputusan SDM atas presensi pulang otomatis.
type AdjustInput struct {
	Action       string `json:"action"`
	ClockOutTime string `json:"clock_out_time"` // HH:MM WIB, untuk set_time
	Note         string `json:"note"`
}

// AdjustResult adalah presensi setelah dikoreksi.
type AdjustResult struct {
	ID            string  `json:"id"`
	WorkDate      string  `json:"work_date"`
	Status        string  `json:"status"`
	ClockOutAt    *string `json:"clock_out_at"`
	WorkedMinutes *int32  `json:"worked_minutes"`
	AdjustedAt    string  `json:"adjusted_at"`
}

// clockOutAt mengubah jam HH:MM menjadi waktu pulang pada tanggal kerja, atau esok harinya bila jamnya
// sebelum jam masuk (shift malam).
func clockOutAt(workDate, clockIn time.Time, hhmm string) (time.Time, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		return time.Time{}, false
	}
	out := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), t.Hour(), t.Minute(), 0, 0, pgconv.WIB)
	if !out.After(clockIn) {
		out = out.AddDate(0, 0, 1)
	}
	return out, out.After(clockIn) && out.Sub(clockIn) <= 24*time.Hour
}

// Adjust mencatat keputusan SDM atas presensi pulang otomatis (D69): sesuai, ganti jam pulang, atau tidak
// hadir. Presensi sendiri tidak bisa dikoreksi, dan tanggal di periode ubar yang terkunci ditolak.
func (s *Service) Adjust(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in AdjustInput) (AdjustResult, error) {
	note := strings.TrimSpace(in.Note)
	fields := map[string]string{}
	switch in.Action {
	case AdjustConfirm, AdjustSetTime:
	case AdjustAbsent:
		if utf8.RuneCountInString(note) < minAbsentNoteLen {
			fields["note"] = "Mohon isi catatan minimal 3 karakter bila Pejuang dinyatakan tidak hadir."
		}
	default:
		fields["action"] = "Pilih sesuai, ganti jam pulang, atau tidak hadir."
	}
	if utf8.RuneCountInString(note) > maxReviewNoteLen {
		fields["note"] = "Catatan maksimal 500 karakter."
	}
	if len(fields) > 0 {
		return AdjustResult{}, httpx.Validation(fields)
	}
	now := s.opt.Now()
	var row store.Attendance
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		current, err := q.GetAttendanceForUpdate(ctx, id)
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		if !current.AutoClockOut || current.AdjustedAt.Valid || !current.ClockInAt.Valid || !current.ClockOutAt.Valid {
			return httpx.NewError(http.StatusConflict, "REQUEST_NOT_PENDING", "Presensi ini sudah dicek.")
		}
		if current.EmployeeID == p.EmployeeID {
			return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Presensi sendiri dicek oleh SDM lain.")
		}
		locked, err := payroll.DateLocked(ctx, q, current.WorkDate.Time, current.WorkDate.Time)
		if err != nil {
			return err
		}
		if locked {
			return payroll.ErrPeriodLocked()
		}
		clockOut, worked, status := current.ClockOutAt, current.WorkedMinutes, current.Status
		switch in.Action {
		case AdjustSetTime:
			out, ok := clockOutAt(current.WorkDate.Time, current.ClockInAt.Time, in.ClockOutTime)
			if !ok || out.After(now) {
				return httpx.Validation(map[string]string{"clock_out_time": "Isi jam pulang setelah jam masuk, paling lama 24 jam sesudahnya dan tidak melewati waktu sekarang."})
			}
			clockOut = pgconv.Time(out)
			worked = pgtype.Int4{Int32: int32(WorkedMinutes(current.ClockInAt.Time, out)), Valid: true}
		case AdjustAbsent:
			status = store.AttStatusAbsent
		}
		row, err = q.AdjustAttendance(ctx, store.AdjustAttendanceParams{
			ID: id, ClockOutAt: clockOut, WorkedMinutes: worked, Status: status,
			AdjustedBy: uuid.NullUUID{UUID: p.UserID, Valid: true}, AdjustedAt: pgconv.Time(now),
			AdjustNote: pgtype.Text{String: note, Valid: note != ""},
		})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.AttendanceAdjusted, Entity: "attendance", EntityID: id, IP: ip,
			Detail: map[string]any{
				"action": in.Action, "clock_out_before": current.ClockOutAt.Time.UTC().Format(time.RFC3339),
				"clock_out_after": clockOut.Time.UTC().Format(time.RFC3339), "status_before": string(current.Status), "status_after": string(status),
			},
		})
	})
	if err != nil {
		return AdjustResult{}, err
	}
	out := AdjustResult{
		ID: row.ID.String(), WorkDate: row.WorkDate.Time.Format(time.DateOnly), Status: string(row.Status),
		ClockOutAt: pgconv.TimeString(row.ClockOutAt), AdjustedAt: row.AdjustedAt.Time.UTC().Format(time.RFC3339),
	}
	if row.WorkedMinutes.Valid {
		out.WorkedMinutes = &row.WorkedMinutes.Int32
	}
	return out, nil
}

// AutoClockOuts menangani GET /v1/admin/attendance/auto-clock-outs.
func (h *Handler) AutoClockOuts(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	rows, total, err := h.svc.AutoClockOuts(r.Context(), page)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, page.Meta(total))
}

// Adjust menangani POST /v1/admin/attendance/{id}/adjust.
func (h *Handler) Adjust(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in AdjustInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Adjust(r.Context(), p, audit.ClientIP(r), id, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
