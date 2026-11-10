package leave

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestEntitlementStatutory(t *testing.T) {
	join := day("2025-11-10")
	cases := []struct {
		name string
		join *time.Time
		ref  string
		want int
	}{
		{"belum 12 bulan", &join, "2026-11-09", 0},
		{"tepat 12 bulan", &join, "2026-11-10", StatutoryDays},
		{"lewat 12 bulan", &join, "2027-03-01", StatutoryDays},
		{"tanggal masuk kosong", nil, "2026-11-10", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, from := Entitlement("statutory", 0, c.join, day(c.ref))
			if got != c.want {
				t.Fatalf("hak = %d, ingin %d", got, c.want)
			}
			if c.join != nil && (from == nil || !from.Equal(day("2026-11-10"))) {
				t.Fatalf("eligible_from = %v, ingin 2026-11-10", from)
			}
			if c.join == nil && from != nil {
				t.Fatalf("eligible_from harus kosong tanpa tanggal masuk")
			}
		})
	}
}

func TestEntitlementCustomAndNone(t *testing.T) {
	if got, _ := Entitlement("custom", 6, nil, day("2026-01-02")); got != 6 {
		t.Fatalf("custom = %d, ingin 6", got)
	}
	if got, _ := Entitlement("none", 0, nil, day("2026-01-02")); got != Unlimited {
		t.Fatalf("none = %d, ingin tanpa batas", got)
	}
}

func TestWorkDaysSkipsWeekendAndHolidays(t *testing.T) {
	// Senin 9 Nov 2026 sampai Ahad 15 Nov 2026, kerja Senin–Jumat, Rabu libur.
	got := WorkDays(day("2026-11-09"), day("2026-11-15"), []int16{1, 2, 3, 4, 5}, map[string]bool{"2026-11-11": true})
	want := []string{"2026-11-09", "2026-11-10", "2026-11-12", "2026-11-13"}
	if len(got) != len(want) {
		t.Fatalf("hari kerja = %v, ingin %v", got, want)
	}
	for i, d := range got {
		if d.Format("2006-01-02") != want[i] {
			t.Fatalf("hari ke-%d = %s, ingin %s", i, d.Format("2006-01-02"), want[i])
		}
	}
	if n := len(WorkDays(day("2026-11-14"), day("2026-11-15"), []int16{1, 2, 3, 4, 5}, nil)); n != 0 {
		t.Fatalf("akhir pekan dihitung %d hari, ingin 0", n)
	}
}
