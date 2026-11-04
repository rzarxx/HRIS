package attendance

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// MonitorItem adalah kondisi absen satu Pejuang pada satu tanggal kerja. Status kosong berarti
// belum ada absen. Scheduled menjawab apakah Pejuang memang dijadwalkan masuk pada tanggal itu.
type MonitorItem struct {
	EmployeeID      string   `json:"employee_id"`
	EmployeeName    string   `json:"employee_name"`
	Nip             string   `json:"nip"`
	SubdivisionName string   `json:"subdivision_name"`
	DivisionID      string   `json:"division_id"`
	DivisionName    string   `json:"division_name"`
	ShiftName       *string  `json:"shift_name"`
	Scheduled       bool     `json:"scheduled"`
	Status          *string  `json:"status"`
	ClockInAt       *string  `json:"clock_in_at"`
	ClockOutAt      *string  `json:"clock_out_at"`
	LateMinutes     int32    `json:"late_minutes"`
	WorkMode        *string  `json:"work_mode"`
	LocationName    *string  `json:"location_name"`
	InLatitude      *float64 `json:"in_latitude"`
	InLongitude     *float64 `json:"in_longitude"`
	OutLatitude     *float64 `json:"out_latitude"`
	OutLongitude    *float64 `json:"out_longitude"`
}

// Monitor mengembalikan semua Pejuang aktif beserta absennya pada satu tanggal kerja.
// Tanggal kosong berarti tanggal kerja saat ini, mengikuti cutoff harian (D31).
func (s *Service) Monitor(ctx context.Context, dateRaw, divisionRaw string) ([]MonitorItem, error) {
	fields := map[string]string{}
	date := WorkDate(s.opt.Now(), Schedule{Start: DayCutoff})
	if dateRaw != "" {
		parsed, err := time.Parse("2006-01-02", dateRaw)
		if err != nil {
			fields["date"] = "Tanggal tidak valid."
		}
		date = parsed
	}
	division := uuid.NullUUID{}
	if divisionRaw != "" {
		id, err := uuid.Parse(divisionRaw)
		if err != nil {
			fields["division_id"] = "Divisi tidak valid."
		}
		division = uuid.NullUUID{UUID: id, Valid: err == nil}
	}
	if len(fields) > 0 {
		return nil, httpx.Validation(fields)
	}
	holiday, err := s.q.IsHoliday(ctx, pgconv.Date(date))
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListAttendanceMonitor(ctx, store.ListAttendanceMonitorParams{WorkDate: pgconv.Date(date), DivisionID: division})
	if err != nil {
		return nil, err
	}
	out := make([]MonitorItem, 0, len(rows))
	for _, r := range rows {
		item := MonitorItem{
			EmployeeID: r.ID.String(), EmployeeName: r.FullName, Nip: r.Nip, SubdivisionName: r.SubdivisionName,
			DivisionID: r.DivisionID.String(), DivisionName: r.DivisionName, ShiftName: pgconv.TextPtr(r.ShiftName),
			Scheduled:  !holiday && r.ShiftName.Valid && IsWorkDay(date, Schedule{WorkDays: r.WorkDays}),
			ClockInAt:  pgconv.TimeString(r.ClockInAt),
			ClockOutAt: pgconv.TimeString(r.ClockOutAt), LateMinutes: r.LateMinutes.Int32, WorkMode: pgconv.TextPtr(r.WorkMode),
			LocationName: pgconv.TextPtr(r.LocationName), InLatitude: floatPtr(r.InLatitude), InLongitude: floatPtr(r.InLongitude),
			OutLatitude: floatPtr(r.OutLatitude), OutLongitude: floatPtr(r.OutLongitude),
		}
		if r.Status.Valid {
			status := string(r.Status.AttStatus)
			item.Status = &status
		}
		out = append(out, item)
	}
	return out, nil
}

func (h *Handler) Monitor(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.Monitor(r.Context(), r.URL.Query().Get("date"), r.URL.Query().Get("division_id"))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}
