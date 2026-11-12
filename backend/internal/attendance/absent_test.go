package attendance

import (
	"testing"
	"time"
)

func TestClosedWorkDate(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"tepat cutoff menutup kemarin", wib(3, 4, 0), date(2)},
		{"jadwal timer 04.20", wib(3, 4, 20), date(2)},
		{"sebelum cutoff kemarin masih terbuka", wib(3, 3, 59), date(1)},
		{"tengah malam belum menutup kemarin", wib(3, 0, 0), date(1)},
		{"malam hari", wib(3, 23, 59), date(2)},
		{"waktu UTC dihitung WIB", time.Date(2026, 11, 2, 21, 30, 0, 0, time.UTC), date(2)}, // 04.30 WIB tanggal 3
		{"lintas bulan", time.Date(2026, 12, 1, 5, 0, 0, 0, time.FixedZone("WIB", 7*3600)), date(30)},
	}
	for _, c := range cases {
		if got := ClosedWorkDate(c.now); !got.Equal(c.want) {
			t.Errorf("%s: %s, ingin %s", c.name, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

// Tanggal yang ditutup tidak boleh lebih awal dari presensi masuk terakhir yang masih sah untuk
// tanggal itu, untuk semua jenis shift.
func TestClosedWorkDateAfterLastClockIn(t *testing.T) {
	for _, s := range []Schedule{regular, night, early, midnight} {
		for minute := 0; minute < 48*60; minute += 5 {
			now := wib(2, 0, 0).Add(time.Duration(minute) * time.Minute)
			if WorkDate(now, s).After(ClosedWorkDate(now)) {
				continue
			}
			t.Fatalf("shift mulai %s: presensi %s masuk tanggal %s yang sudah ditutup", s.Start, now, WorkDate(now, s).Format("2006-01-02"))
		}
	}
}

func TestAbsentRange(t *testing.T) {
	from, to, err := AbsentRange(wib(10, 4, 30), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equal(date(3)) || !to.Equal(date(9)) {
		t.Fatalf("rentang %s s.d. %s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	from, to, err = AbsentRange(wib(10, 4, 30), 1)
	if err != nil || !from.Equal(to) || !to.Equal(date(9)) {
		t.Fatalf("satu hari: %s s.d. %s, %v", from, to, err)
	}
	for _, days := range []int{0, -1, MaxAbsentLookbackDays + 1} {
		if _, _, err := AbsentRange(wib(10, 4, 30), days); err == nil {
			t.Errorf("days=%d harus ditolak", days)
		}
	}
}
