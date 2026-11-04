package attendance

import (
	"slices"
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

func TestArrangementActiveOn(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	end := day(13)
	friday := Arrangement{StartsOn: day(2), EndsOn: &end, Weekdays: []int16{5}}
	cases := []struct {
		name string
		date time.Time
		a    Arrangement
		want bool
	}{
		{"sebelum mulai", day(1), Arrangement{StartsOn: day(2)}, false},
		{"hari mulai", day(2), Arrangement{StartsOn: day(2)}, true},
		{"tanpa akhir", day(2).AddDate(5, 0, 0), Arrangement{StartsOn: day(2)}, true},
		{"Jumat dalam rentang", day(6), friday, true},
		{"bukan Jumat", day(5), friday, false},
		{"hari terakhir", day(13), friday, true},
		{"lewat akhir", day(20), friday, false},
		{"Minggu bernilai 7", day(8), Arrangement{StartsOn: day(2), Weekdays: []int16{7}}, true},
		// Zona WIB malam hari tetap dibaca sebagai tanggalnya sendiri.
		{"tanggal WIB", time.Date(2026, 11, 6, 23, 0, 0, 0, pgconv.WIB), friday, true},
	}
	for _, c := range cases {
		if got := c.a.ActiveOn(c.date); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestArrangementOverlaps(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	until := func(d int) *time.Time { e := day(d); return &e }
	cases := []struct {
		name string
		a, b Arrangement
		want bool
	}{
		{"rentang terpisah", Arrangement{StartsOn: day(2), EndsOn: until(5)}, Arrangement{StartsOn: day(6), EndsOn: until(9)}, false},
		{"bersentuhan satu hari", Arrangement{StartsOn: day(2), EndsOn: until(6)}, Arrangement{StartsOn: day(6)}, true},
		{"hari berbeda tanpa akhir", Arrangement{StartsOn: day(2), Weekdays: []int16{1, 2}}, Arrangement{StartsOn: day(2), Weekdays: []int16{5}}, false},
		{"hari sama tanpa akhir", Arrangement{StartsOn: day(2), Weekdays: []int16{5}}, Arrangement{StartsOn: day(20), Weekdays: []int16{5, 6}}, true},
		{"satu hari Senin dan izin Jumat", Arrangement{StartsOn: day(2), EndsOn: until(2)}, Arrangement{StartsOn: day(1), Weekdays: []int16{5}}, false},
		{"izin hari kosong dalam rentang", Arrangement{StartsOn: day(2), EndsOn: until(2), Weekdays: []int16{5}}, Arrangement{StartsOn: day(2), EndsOn: until(2), Weekdays: []int16{5}}, false},
	}
	for _, c := range cases {
		if got := c.a.Overlaps(c.b); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if got := c.b.Overlaps(c.a); got != c.want {
			t.Errorf("%s (dibalik): got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSoftFlags(t *testing.T) {
	now := wib(2, 9, 0)
	office := Point{Latitude: -6.2001, Longitude: 106.8166}
	far := Point{Latitude: -7.7956, Longitude: 110.3695}  // sekitar 430 km
	near := Point{Latitude: -6.2006, Longitude: 106.8166} // sekitar 55 m
	base := RiskInput{Now: now, Point: office, AccuracyM: 10, CapturedAt: now}
	approved := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	cases := []struct {
		name  string
		edit  func(*RiskInput)
		want  []string
		score int
	}{
		{"bersih", func(*RiskInput) {}, nil, 0},
		{"akurasi tepat batas", func(in *RiskInput) { in.AccuracyM = 50 }, nil, 0},
		{"akurasi rendah", func(in *RiskInput) { in.AccuracyM = 50.5 }, []string{flagAccuracyLow}, 15},
		{"jam tepat batas", func(in *RiskInput) { in.CapturedAt = now.Add(-5 * time.Minute) }, nil, 0},
		{"jam HP terlambat", func(in *RiskInput) { in.CapturedAt = now.Add(-6 * time.Minute) }, []string{flagClockSkew}, 20},
		{"jam HP mendahului", func(in *RiskInput) { in.CapturedAt = now.Add(6 * time.Minute) }, []string{flagClockSkew}, 20},
		{"perangkat baru", func(in *RiskInput) { in.DeviceApprovedAt = approved(23 * time.Hour) }, []string{flagNewDevice}, 10},
		{"perangkat lama", func(in *RiskInput) { in.DeviceApprovedAt = approved(24 * time.Hour) }, nil, 0},
		{"melompat", func(in *RiskInput) { in.Previous = &PointAt{Point: far, At: now.Add(-time.Hour)} }, []string{flagLocationJump}, 40},
		{"perjalanan wajar", func(in *RiskInput) { in.Previous = &PointAt{Point: far, At: now.Add(-8 * time.Hour)} }, nil, 0},
		{"waktu sama tetapi jauh", func(in *RiskInput) { in.Previous = &PointAt{Point: far, At: now} }, []string{flagLocationJump}, 40},
		{"gangguan GPS", func(in *RiskInput) { in.Previous = &PointAt{Point: near, At: now.Add(-time.Second)} }, nil, 0},
	}
	for _, c := range cases {
		in := base
		c.edit(&in)
		got := SoftFlags(in)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if score := RiskScore(got); score != c.score {
			t.Errorf("%s: skor %d, want %d", c.name, score, c.score)
		}
	}
	if score := RiskScore([]string{flagUnverified, flagOffSchedule}); score != 0 {
		t.Errorf("penanda catatan bernilai %d", score)
	}
}
