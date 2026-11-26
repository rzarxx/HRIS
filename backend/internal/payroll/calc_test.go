package payroll

import (
	"testing"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func nd(s string) decimal.NullDecimal { return decimal.NewNullDecimal(d(s)) }

func comp(id, code, kind, method string, mod func(*Component)) Component {
	c := Component{ID: id, Code: code, Name: code, Kind: kind, Method: method, ShowOnSlip: true}
	if mod != nil {
		mod(&c)
	}
	return c
}

func amountOf(r Result, code string) string {
	for _, it := range r.Items {
		if it.Code == code {
			return it.Amount.String()
		}
	}
	return "-"
}

func TestCalculateAllMethods(t *testing.T) {
	att := Attendance{PresentDays: 20, LateCount: 3, LateMinutes: 45, AbsentDays: 2, UnpaidLeaveDays: 1}
	comps := []Component{
		comp("1", "TUNJ_TETAP", KindEarning, MethodFixed, func(c *Component) { c.Rate = nd("200000") }),
		comp("2", "TUNJ_JABATAN", KindEarning, MethodPerEmployee, nil),
		comp("3", "UANG_MAKAN", KindEarning, MethodPerPresentDay, func(c *Component) { c.Rate = nd("15000") }),
		comp("4", "TELAT", KindDeduction, MethodPerLate, func(c *Component) { c.Rate = nd("5000") }),
		comp("5", "TELAT_MENIT", KindDeduction, MethodPerLateMinute, func(c *Component) { c.Rate = nd("100") }),
		comp("6", "ALPHA", KindDeduction, MethodPerAbsentDay, func(c *Component) { c.Divisor = nd("25") }),
		comp("7", "CUTI_TB", KindDeduction, MethodPerUnpaidLeaveDay, func(c *Component) { c.Rate = nd("50000") }),
		comp("8", "BPJS", KindDeduction, MethodPercentOfBase, func(c *Component) { c.Percent = nd("1") }),
		comp("9", "LEMBUR", KindEarning, MethodPerUnit, func(c *Component) { c.Rate = nd("20000") }),
		comp("10", "PINJAMAN", KindDeduction, MethodManual, nil),
		comp("11", "PPH21", KindDeduction, MethodPercentOfGross, func(c *Component) { c.Percent = nd("2.5") }),
	}
	perEmp := map[string]decimal.Decimal{"2": d("300000")}
	inputs := map[string]Input{"9": {Quantity: nd("4.5")}, "10": {Amount: nd("250000")}}
	r := Calculate(d("5000000"), att, comps, perEmp, inputs, true)

	want := map[string]string{
		BaseCode:       "5000000",
		"TUNJ_TETAP":   "200000",
		"TUNJ_JABATAN": "300000",
		"UANG_MAKAN":   "300000",
		"TELAT":        "15000",
		"TELAT_MENIT":  "4500",
		"ALPHA":        "400000", // 5.000.000 / 25 x 2
		"CUTI_TB":      "50000",
		"BPJS":         "50000",
		"LEMBUR":       "90000",
		"PINJAMAN":     "250000",
		"PPH21":        "147250", // 2,5% x (5.000.000 + 200.000 + 300.000 + 300.000 + 90.000)
	}
	for code, amt := range want {
		if got := amountOf(r, code); got != amt {
			t.Errorf("%s = %s, ingin %s", code, got, amt)
		}
	}
	if r.Gross.String() != "5890000" || r.Deductions.String() != "916750" || r.Net.String() != "4973250" {
		t.Errorf("bruto %s potongan %s bersih %s", r.Gross, r.Deductions, r.Net)
	}
}

func TestCalculateSkipsEmptyAndClampsNet(t *testing.T) {
	comps := []Component{
		comp("1", "TUNJ_JABATAN", KindEarning, MethodPerEmployee, nil),                               // tanpa nilai untuk Pejuang ini
		comp("2", "LEMBUR", KindEarning, MethodPerUnit, func(c *Component) { c.Rate = nd("20000") }), // tanpa isian
		comp("3", "UANG_MAKAN", KindEarning, MethodPerPresentDay, func(c *Component) { c.Rate = nd("15000") }),
		comp("4", "PINJAMAN", KindDeduction, MethodManual, nil),
	}
	r := Calculate(d("1000000"), Attendance{}, comps, nil, map[string]Input{"4": {Amount: nd("1500000")}}, true)
	if len(r.Items) != 2 {
		t.Fatalf("item = %+v, ingin ubar pokok dan pinjaman saja", r.Items)
	}
	if !r.Net.IsZero() || !r.NetClamped {
		t.Errorf("bersih = %s, clamped = %v", r.Net, r.NetClamped)
	}
	r = Calculate(d("1000000"), Attendance{}, comps, nil, map[string]Input{"4": {Amount: nd("1500000")}}, false)
	if r.Net.String() != "-500000" || r.NetClamped {
		t.Errorf("tanpa batas 0: bersih = %s", r.Net)
	}
}

func TestCalculateAbsentDefaultDivisorAndRounding(t *testing.T) {
	comps := []Component{comp("1", "ALPHA", KindDeduction, MethodPerAbsentDay, nil)}
	r := Calculate(d("1000000"), Attendance{AbsentDays: 1}, comps, nil, nil, true)
	// 1.000.000 / 26 = 38461,53... dibulatkan ke rupiah.
	if got := amountOf(r, "ALPHA"); got != "38462" {
		t.Errorf("alpha = %s", got)
	}
}

func TestCalculateOrderFollowsSortOrder(t *testing.T) {
	comps := []Component{
		comp("1", "B", KindEarning, MethodFixed, func(c *Component) { c.Rate = nd("1"); c.SortOrder = 2 }),
		comp("2", "A", KindEarning, MethodFixed, func(c *Component) { c.Rate = nd("1"); c.SortOrder = 1 }),
	}
	r := Calculate(d("0"), Attendance{}, comps, nil, nil, true)
	if len(r.Items) != 3 || r.Items[1].Code != "A" || r.Items[2].Code != "B" {
		t.Errorf("urutan = %+v", r.Items)
	}
}
