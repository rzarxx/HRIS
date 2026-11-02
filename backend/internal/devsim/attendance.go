// Package devsim mensimulasikan presensi harian selama pengembangan, sampai aplikasi HP
// (Fase 3) mengirim presensi nyata. Hanya berjalan bila DEV_SIMULATE_ATTENDANCE aktif.
package devsim

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// AttendanceStart adalah hari pertama data simulasi.
var AttendanceStart = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

func seededRand(s string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return rand.New(rand.NewSource(int64(h.Sum64())))
}

func wibTime(date time.Time, minutes int) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), minutes/60, minutes%60, 0, 0, pgconv.WIB)
}

// isWorkDay memakai konvensi shifts.work_days: 1=Senin..7=Minggu.
func isWorkDay(date time.Time, workDays []int16) bool {
	day := int16(date.Weekday())
	if day == 0 {
		day = 7
	}
	for _, d := range workDays {
		if d == day {
			return true
		}
	}
	return false
}

// FillAttendance mengisi hari kerja yang belum punya baris, termasuk baris absent
// karena alpha harus berupa baris nyata agar terhitung di laporan. Hasilnya deterministik
// per Pejuang dan tanggal. Mengembalikan jumlah baris baru.
func FillAttendance(ctx context.Context, q *store.Queries, now time.Time) (int, error) {
	shift, err := q.GetDefaultShift(ctx)
	if err != nil {
		return 0, fmt.Errorf("shift: %w", err)
	}
	employees, err := q.ListActiveEmployeesForAttendance(ctx)
	if err != nil {
		return 0, err
	}
	keys, err := q.ListAttendanceKeysSince(ctx, pgconv.Date(AttendanceStart))
	if err != nil {
		return 0, err
	}
	existing := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		existing[k.EmployeeID.String()+"|"+k.WorkDate.Time.Format("2006-01-02")] = struct{}{}
	}

	shiftStart := int(shift.StartTime.Microseconds / 60_000_000)
	shiftEnd := int(shift.EndTime.Microseconds / 60_000_000)
	tolerance := int(shift.LateToleranceMinutes)
	today := pgconv.WorkDate(now)
	wibNow := now.In(pgconv.WIB)
	minutesNow := wibNow.Hour()*60 + wibNow.Minute()
	shiftID := uuid.NullUUID{UUID: shift.ID, Valid: true}

	created := 0
	for _, emp := range employees {
		start := AttendanceStart
		if emp.JoinDate.Valid && emp.JoinDate.Time.After(start) {
			start = emp.JoinDate.Time
		}
		profile := seededRand(emp.ID.String())
		lateProb := 0.03 + profile.Float64()*0.22
		absentProb := profile.Float64() * 0.05

		for date := start; !date.After(today); date = date.AddDate(0, 0, 1) {
			dateKey := date.Format("2006-01-02")
			if !isWorkDay(date, shift.WorkDays) {
				continue
			}
			if _, ok := existing[emp.ID.String()+"|"+dateKey]; ok {
				continue
			}
			row := simulateDay(emp.ID, date, shiftID, today, minutesNow, shiftStart, shiftEnd, tolerance, lateProb, absentProb)
			if row == nil {
				continue
			}
			affected, err := q.InsertAttendanceIfMissing(ctx, *row)
			if err != nil {
				return created, err
			}
			created += int(affected)
		}
	}
	return created, nil
}

// simulateDay mengembalikan nil bila hari ini belum waktunya tercatat.
func simulateDay(employeeID uuid.UUID, date time.Time, shiftID uuid.NullUUID, today time.Time, minutesNow, shiftStart, shiftEnd, tolerance int, lateProb, absentProb float64) *store.InsertAttendanceIfMissingParams {
	r := seededRand(employeeID.String() + "|" + date.Format("2006-01-02"))
	isToday := date.Equal(today)
	base := store.InsertAttendanceIfMissingParams{EmployeeID: employeeID, WorkDate: pgconv.Date(date), ShiftID: shiftID}

	roll := r.Float64()
	switch {
	case roll < absentProb:
		if isToday && minutesNow < shiftEnd {
			return nil // alpha baru dipastikan setelah jam pulang
		}
		base.Status = store.AttStatusAbsent
		return &base
	case roll < absentProb+0.04:
		base.Status = store.AttStatusLeave
		return &base
	}

	late := r.Float64() < lateProb
	var inMinute int
	if late {
		spread := r.Float64()
		inMinute = shiftStart + tolerance + 1 + int(spread*spread*70)
	} else {
		inMinute = shiftStart - 40 + r.Intn(40+tolerance)
	}
	outMinute := shiftEnd + r.Intn(60)
	if isToday && minutesNow < inMinute {
		return nil // belum waktunya absen masuk
	}
	base.ClockInAt = pgconv.Time(wibTime(date, inMinute))
	if !isToday || minutesNow >= outMinute {
		base.ClockOutAt = pgconv.Time(wibTime(date, outMinute))
	}
	base.RiskScore = int32(r.Intn(20))
	if late {
		base.Status = store.AttStatusLate
		base.LateMinutes = int32(inMinute - shiftStart)
	} else {
		base.Status = store.AttStatusPresent
	}
	return &base
}
