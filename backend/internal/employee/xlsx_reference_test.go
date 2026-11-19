package employee

import (
	"slices"
	"testing"
)

func TestMakeCode(t *testing.T) {
	taken := map[string]bool{"DAKWAH": true}
	for _, tc := range []struct {
		name string
		max  int
		want string
	}{
		{"Dakwah", 10, "DAKWAH-2"},
		{"Dakwah", 10, "DAKWAH-3"},
		{"Kajian & Tahfidz Anak", 12, "KAJIAN-TAHFI"},
		{"Ü", 10, "UNIT"},
		{"Cabang Cirebon Timur Raya Sekali", 20, "CABANG-CIREBON-TIMUR"},
	} {
		if got := makeCode(tc.name, tc.max, taken); got != tc.want {
			t.Errorf("makeCode(%q) = %q, ingin %q", tc.name, got, tc.want)
		}
	}
}

func TestParseWorkDays(t *testing.T) {
	for raw, want := range map[string][]int16{
		"":                   {1, 2, 3, 4, 5},
		"1-5":                {1, 2, 3, 4, 5},
		"Senin-Sabtu":        {1, 2, 3, 4, 5, 6},
		"senin, rabu, jumat": {1, 3, 5},
		"1-3,7":              {1, 2, 3, 7},
		"Ahad":               {7},
	} {
		got, ok := parseWorkDays(raw)
		if !ok || !slices.Equal(got, want) {
			t.Errorf("parseWorkDays(%q) = %v %v, ingin %v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"8", "Jumat-Senin", "libur"} {
		if _, ok := parseWorkDays(raw); ok {
			t.Errorf("parseWorkDays(%q) seharusnya ditolak", raw)
		}
	}
	if got := workDaysString([]int16{1, 2, 3, 4, 5, 7}); got != "1-5,7" {
		t.Errorf("workDaysString = %q", got)
	}
}

func TestParseClockCell(t *testing.T) {
	for raw, minutes := range map[string]int64{"07:30": 450, "7.30": 450, "0.3125": 450, "23:59": 1439} {
		got, ok := parseClockCell(raw)
		if !ok || got.Microseconds != minutes*60_000_000 {
			t.Errorf("parseClockCell(%q) = %v %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"24:00", "7", "1.5", "pagi"} {
		if _, ok := parseClockCell(raw); ok {
			t.Errorf("parseClockCell(%q) seharusnya ditolak", raw)
		}
	}
}
