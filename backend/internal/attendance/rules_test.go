package attendance

import (
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

func wib(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, pgconv.WIB)
}

func date(day int) time.Time { return time.Date(2026, time.November, day, 0, 0, 0, 0, time.UTC) }

var (
	regular  = Schedule{Start: 8 * time.Hour, End: 17 * time.Hour, ToleranceMinutes: 10, WorkDays: []int16{1, 2, 3, 4, 5}}
	night    = Schedule{Start: 22 * time.Hour, End: 6 * time.Hour, ToleranceMinutes: 10, WorkDays: []int16{1, 2, 3, 4, 5}}
	early    = Schedule{Start: 3 * time.Hour, End: 12 * time.Hour, ToleranceMinutes: 0, WorkDays: []int16{1, 2, 3, 4, 5}}
	midnight = Schedule{Start: 0, End: 8 * time.Hour, ToleranceMinutes: 0, WorkDays: []int16{1, 2, 3, 4, 5}}
)

func TestWorkDate(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		s    Schedule
		want time.Time
	}{
		{"siang biasa", wib(3, 8, 0), regular, date(3)},
		{"tepat cutoff masuk hari ini", wib(3, 4, 0), regular, date(3)},
		{"sebelum cutoff masuk kemarin", wib(3, 3, 59), regular, date(2)},
		{"shift malam telat lewat tengah malam", wib(3, 0, 30), night, date(2)},
		{"shift malam masuk sebelum tengah malam", wib(2, 22, 0), night, date(2)},
		{"shift dini hari tetap hari ini", wib(3, 2, 55), early, date(3)},
		{"datang awal untuk shift tengah malam", wib(2, 23, 50), midnight, date(3)},
		{"terlalu awal untuk shift tengah malam", wib(2, 21, 59), midnight, date(2)},
		{"shift tengah malam tepat waktu", wib(3, 0, 5), midnight, date(3)},
		{"waktu UTC tetap dihitung WIB", time.Date(2026, 11, 2, 17, 30, 0, 0, time.UTC), regular, date(2)}, // 00.30 WIB tanggal 3, sebelum cutoff
	}
	for _, c := range cases {
		if got := WorkDate(c.now, c.s); !got.Equal(c.want) {
			t.Errorf("%s: %s, ingin %s", c.name, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

func TestIsWorkDay(t *testing.T) {
	if !IsWorkDay(date(2), regular) { // Senin
		t.Error("Senin harus hari kerja")
	}
	if IsWorkDay(date(1), regular) { // Minggu = 7
		t.Error("Minggu bukan hari kerja")
	}
	if !IsWorkDay(date(1), Schedule{WorkDays: []int16{7}}) {
		t.Error("Minggu harus dikenali sebagai 7")
	}
}

func TestLateMinutes(t *testing.T) {
	cases := []struct {
		now  time.Time
		want int
	}{
		{wib(2, 7, 30), 0},
		{wib(2, 8, 0), 0},
		{wib(2, 8, 10), 0},
		{wib(2, 8, 11), 11},
		{wib(2, 9, 0), 60},
	}
	for _, c := range cases {
		if got := LateMinutes(c.now, date(2), regular); got != c.want {
			t.Errorf("%s: %d, ingin %d", c.now.Format("15:04"), got, c.want)
		}
	}
	if got := LateMinutes(wib(3, 0, 30), date(2), night); got != 150 {
		t.Errorf("shift malam: %d, ingin 150", got)
	}
}

func TestCloseDeadline(t *testing.T) {
	if got := CloseDeadline(date(2), &regular); !got.Equal(wib(3, 4, 0)) {
		t.Errorf("shift biasa: %s", got)
	}
	if got := CloseDeadline(date(2), nil); !got.Equal(wib(3, 4, 0)) {
		t.Errorf("tanpa shift: %s", got)
	}
	// Shift malam berakhir 06.00, melewati cutoff, jadi batasnya ikut mundur.
	if got := CloseDeadline(date(2), &night); !got.Equal(wib(3, 10, 0)) {
		t.Errorf("shift malam: %s", got)
	}
}

func TestWorkedMinutes(t *testing.T) {
	if got := WorkedMinutes(wib(2, 22, 0), wib(3, 6, 10)); got != 490 {
		t.Errorf("lewat tengah malam: %d", got)
	}
	if got := WorkedMinutes(wib(2, 8, 0), wib(2, 7, 0)); got != 0 {
		t.Errorf("waktu terbalik: %d", got)
	}
}
