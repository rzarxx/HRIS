// Package leave mengelola cuti dan izin (MASTER.md F13, D47).
package leave

import (
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
)

const (
	// StatutoryDays adalah hak cuti tahunan menurut UU Ketenagakerjaan.
	StatutoryDays = 12
	// Unlimited menandai jenis tanpa saldo.
	Unlimited = -1

	maxRangeDays = 92 // cukup untuk cuti melahirkan tiga bulan
	maxBackDays  = 30 // sakit atau izin boleh dicatat belakangan
)

// Entitlement menghitung jatah satu jenis untuk tahun tanggal ref. Untuk statutory, hak 12 hari
// berlaku setelah 12 bulan bekerja; tanggal masuk kosong berarti belum bisa dihitung (0 hari).
func Entitlement(quotaKind string, annual int, joinDate *time.Time, ref time.Time) (days int, eligibleFrom *time.Time) {
	switch quotaKind {
	case "statutory":
		if joinDate == nil {
			return 0, nil
		}
		from := joinDate.AddDate(1, 0, 0)
		if ref.Before(from) {
			return 0, &from
		}
		return StatutoryDays, &from
	case "custom":
		return annual, nil
	default:
		return Unlimited, nil
	}
}

// WorkDays menghitung hari kerja dalam rentang menurut hari kerja shift, tanpa hari libur.
func WorkDays(start, end time.Time, workDays []int16, holidays map[string]bool) []time.Time {
	var out []time.Time
	schedule := attendance.Schedule{WorkDays: workDays}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if attendance.IsWorkDay(d, schedule) && !holidays[d.Format("2006-01-02")] {
			out = append(out, d)
		}
	}
	return out
}
