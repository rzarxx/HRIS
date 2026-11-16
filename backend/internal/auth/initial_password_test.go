package auth

import (
	"testing"
	"time"
)

func TestInitialPassword(t *testing.T) {
	// 31 Des 2026 pukul 20.00 UTC sudah 1 Jan 2027 di WIB.
	newYearWIB := time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"Siti Nurhaliza", mid, "@Siti2026"},
		{"SITI NURHALIZA", mid, "@Siti2026"},
		{"M. Rizal Fauzi", mid, "@Rizal2026"},
		{"Muhammad", mid, "@Muhammad2026"},
		{"  'Aisyah  binti Umar ", mid, "@Aisyah2026"},
		{"Ahmad", newYearWIB, "@Ahmad2027"},
		{"A. B.", mid, "@Pejuang2026"},
	}
	for _, c := range cases {
		if got := InitialPassword(c.name, c.now); got != c.want {
			t.Errorf("InitialPassword(%q) = %q, mau %q", c.name, got, c.want)
		}
	}
}
