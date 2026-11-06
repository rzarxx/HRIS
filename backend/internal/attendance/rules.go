package attendance

import (
	"slices"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

const (
	// DayCutoff adalah cutoff harian (D31). Nilai awal ini menjadi pengaturan SDM di Fase 5.
	DayCutoff = 4 * time.Hour
	// overnightCloseGrace memberi waktu absen pulang setelah shift malam berakhir,
	// karena shift malam bisa berakhir melewati cutoff harian.
	overnightCloseGrace = 4 * time.Hour
	// earlyClockInWindow adalah batas datang lebih awal untuk shift yang dimulai sekitar tengah malam.
	earlyClockInWindow = 2 * time.Hour
)

// Schedule adalah jadwal shift. Start dan End dihitung dari tengah malam WIB.
type Schedule struct {
	Start            time.Duration
	End              time.Duration
	ToleranceMinutes int
	WorkDays         []int16 // 1=Senin sampai 7=Ahad
}

func at(date time.Time, offset time.Duration) time.Time {
	y, m, d := date.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, pgconv.WIB).Add(offset)
}

// WorkDate menentukan tanggal kerja untuk absen masuk. Absen sebelum cutoff harian masuk ke
// hari sebelumnya, kecuali shift-nya memang dimulai sebelum cutoff (mis. shift pukul 03.00).
// Shift seperti itu juga menerima absen yang datang lebih awal sebelum tengah malam.
func WorkDate(now time.Time, s Schedule) time.Time {
	date := pgconv.WorkDate(now)
	sinceMidnight := now.In(pgconv.WIB).Sub(at(date, 0))
	if sinceMidnight < DayCutoff && s.Start >= DayCutoff {
		return date.AddDate(0, 0, -1)
	}
	if s.Start < DayCutoff && sinceMidnight >= 24*time.Hour-earlyClockInWindow+s.Start {
		return date.AddDate(0, 0, 1)
	}
	return date
}

func isoWeekday(date time.Time) int16 {
	day := int16(date.Weekday())
	if day == 0 {
		return 7
	}
	return day
}

// IsWorkDay menjawab apakah tanggal termasuk hari kerja shift.
func IsWorkDay(date time.Time, s Schedule) bool {
	return slices.Contains(s.WorkDays, isoWeekday(date))
}

// Arrangement adalah aturan tanggal izin kerja. Weekdays kosong berarti setiap hari.
type Arrangement struct {
	StartsOn time.Time
	EndsOn   *time.Time // kosong berarti tanpa akhir
	Weekdays []int16
}

// calendarDay membuang jam dan zona waktu, karena tanggal dari database dan dari WorkDate
// memakai zona yang berbeda.
func calendarDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ActiveOn menjawab apakah izin berlaku pada tanggal itu.
func (a Arrangement) ActiveOn(date time.Time) bool {
	day := calendarDay(date)
	if day.Before(calendarDay(a.StartsOn)) {
		return false
	}
	if a.EndsOn != nil && day.After(calendarDay(*a.EndsOn)) {
		return false
	}
	return len(a.Weekdays) == 0 || slices.Contains(a.Weekdays, isoWeekday(day))
}

// Overlaps menjawab apakah dua izin berlaku pada tanggal yang sama. Tujuh hari berturut-turut
// sudah mencakup semua hari dalam seminggu, jadi izin tanpa akhir pun cukup diperiksa sejauh itu.
func (a Arrangement) Overlaps(b Arrangement) bool {
	day := calendarDay(a.StartsOn)
	if start := calendarDay(b.StartsOn); start.After(day) {
		day = start
	}
	for range 7 {
		if a.ActiveOn(day) && b.ActiveOn(day) {
			return true
		}
		day = day.AddDate(0, 0, 1)
	}
	return false
}

// LateMinutes menghitung keterlambatan dari jam masuk shift. Selama masih dalam toleransi
// hasilnya 0; lewat toleransi, seluruh selisih dari jam masuk dihitung.
func LateMinutes(now, date time.Time, s Schedule) int {
	minutes := int(now.Sub(at(date, s.Start)) / time.Minute)
	if minutes <= s.ToleranceMinutes {
		return 0
	}
	return minutes
}

// CloseDeadline adalah batas absen pulang untuk absen masuk di tanggal itu. Lewat batas ini,
// jam kerja tidak dihitung otomatis dan SDM yang mengoreksi (D31). s kosong bila absen tanpa shift.
func CloseDeadline(date time.Time, s *Schedule) time.Time {
	deadline := at(date.AddDate(0, 0, 1), DayCutoff)
	if s != nil && s.End <= s.Start {
		if overnight := at(date.AddDate(0, 0, 1), s.End+overnightCloseGrace); overnight.After(deadline) {
			return overnight
		}
	}
	return deadline
}

// Nilai awal sinyal risiko (MASTER.md §5.4). Ambang skor menjadi pengaturan di Fase 5.
const (
	riskReviewMin = 30
	riskRejectMin = 60

	accuracyLowM    = 50
	locationJumpKmH = 120
	clockSkewLimit  = 5 * time.Minute
	newDeviceWindow = 24 * time.Hour
	// Di bawah jarak ini, selisih dianggap gangguan GPS. Absen masuk dan pulang yang berdekatan
	// waktunya bisa terbaca sangat cepat walau Pejuang tidak berpindah.
	locationJumpMinM = 1000

	flagAccuracyLow  = "accuracy_low"
	flagLocationJump = "location_jump"
	flagClockSkew    = "clock_skew"
	flagNewDevice    = "new_device_recent"
)

var softPoints = map[string]int{
	flagAccuracyLow:    15,
	flagLocationJump:   40,
	flagClockSkew:      20,
	flagNewDevice:      10,
	flagFaceBorderline: 20,
}

// PointAt adalah titik absen beserta waktunya.
type PointAt struct {
	Point Point
	At    time.Time
}

// RiskInput adalah bahan penilaian sinyal lunak satu upaya absen.
type RiskInput struct {
	Now              time.Time
	Point            Point
	AccuracyM        float64
	CapturedAt       time.Time
	DeviceApprovedAt *time.Time
	Previous         *PointAt // absen terakhir yang tercatat
}

// SoftFlags mengembalikan sinyal lunak yang menyala.
func SoftFlags(in RiskInput) []string {
	var flags []string
	if in.AccuracyM > accuracyLowM {
		flags = append(flags, flagAccuracyLow)
	}
	if in.Previous != nil {
		meters := DistanceM(in.Previous.Point, in.Point)
		if hours := in.Now.Sub(in.Previous.At).Hours(); meters >= locationJumpMinM && (hours <= 0 || meters/1000/hours > locationJumpKmH) {
			flags = append(flags, flagLocationJump)
		}
	}
	if skew := in.Now.Sub(in.CapturedAt); skew > clockSkewLimit || skew < -clockSkewLimit {
		flags = append(flags, flagClockSkew)
	}
	if in.DeviceApprovedAt != nil && in.Now.Sub(*in.DeviceApprovedAt) < newDeviceWindow {
		flags = append(flags, flagNewDevice)
	}
	return flags
}

// RiskScore menjumlahkan poin. Penanda tanpa poin, seperti off_schedule, bernilai 0.
func RiskScore(flags []string) int {
	total := 0
	for _, f := range flags {
		total += softPoints[f]
	}
	return total
}

// WorkedMinutes menghitung jam kerja dalam menit penuh.
func WorkedMinutes(in, out time.Time) int {
	minutes := int(out.Sub(in) / time.Minute)
	if minutes < 0 {
		return 0
	}
	return minutes
}
