package report

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculate(t *testing.T) {
	cases := []struct {
		name                          string
		base                          string
		absent, late                  int64
		wantAbsent, wantLate, wantNet string
	}{
		{"tanpa potongan", "5200000", 0, 0, "0", "0", "5200000"},
		{"alpha satu hari", "5200000", 1, 0, "200000", "0", "5000000"},
		{"telat dua kali", "5200000", 0, 2, "0", "10000", "5190000"},
		{"pembulatan alpha", "4250000", 1, 1, "163462", "5000", "4081538"},
		{"upah bersih tidak negatif", "100000", 30, 10, "115385", "50000", "0"},
	}
	for _, c := range cases {
		base := decimal.RequireFromString(c.base)
		absent, late, net := Calculate(base, c.absent, c.late)
		if !absent.Equal(decimal.RequireFromString(c.wantAbsent)) ||
			!late.Equal(decimal.RequireFromString(c.wantLate)) ||
			!net.Equal(decimal.RequireFromString(c.wantNet)) {
			t.Errorf("%s: dapat alpha=%s telat=%s bersih=%s", c.name, absent, late, net)
		}
	}
}
