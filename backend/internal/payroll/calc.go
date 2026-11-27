// Package payroll menghitung ubar dari komponen yang diatur SDM (D66), mengelola periode ubar dengan
// maker-checker (D67), dan menerbitkan slip ubar (D68).
package payroll

import (
	"sort"

	"github.com/shopspring/decimal"
)

// Jenis dan metode komponen; nilainya sama dengan enum di database.
const (
	KindEarning   = "earning"
	KindDeduction = "deduction"

	MethodFixed             = "fixed"
	MethodPerEmployee       = "per_employee"
	MethodPerPresentDay     = "per_present_day"
	MethodPerLate           = "per_late"
	MethodPerLateMinute     = "per_late_minute"
	MethodPerAbsentDay      = "per_absent_day"
	MethodPerUnpaidLeaveDay = "per_unpaid_leave_day"
	MethodPercentOfBase     = "percent_of_base"
	MethodPercentOfGross    = "percent_of_gross"
	MethodPerUnit           = "per_unit"
	MethodManual            = "manual"

	// BaseCode adalah kode item ubar pokok di slip.
	BaseCode = "UBAR_POKOK"
)

// Methods berurutan seperti pilihan di panel.
var Methods = []string{
	MethodFixed, MethodPerEmployee, MethodPerPresentDay, MethodPerLate, MethodPerLateMinute, MethodPerAbsentDay,
	MethodPerUnpaidLeaveDay, MethodPercentOfBase, MethodPercentOfGross, MethodPerUnit, MethodManual,
}

// DefaultDivisor dipakai per_absent_day dan per_unpaid_leave_day tanpa rate dan tanpa pembagi (hari kerja
// sebulan yang lazim). SDM dapat mengubahnya per komponen.
var DefaultDivisor = decimal.NewFromInt(26)

// Component adalah salinan komponen yang dipakai menghitung (juga disimpan di periode saat generate).
type Component struct {
	ID         string              `json:"id"`
	Code       string              `json:"code"`
	Name       string              `json:"name"`
	Kind       string              `json:"kind"`
	Method     string              `json:"method"`
	Rate       decimal.NullDecimal `json:"rate"`
	Percent    decimal.NullDecimal `json:"percent"`
	Divisor    decimal.NullDecimal `json:"divisor"`
	ShowOnSlip bool                `json:"show_on_slip"`
	SortOrder  int32               `json:"sort_order"`
}

// Attendance adalah ringkasan presensi satu Pejuang dalam rentang periode.
type Attendance struct {
	WorkDays        decimal.Decimal `json:"work_days"`
	PresentDays     int64           `json:"present_days"`
	LateCount       int64           `json:"late_count"`
	LateMinutes     int64           `json:"late_minutes"`
	AbsentDays      int64           `json:"absent_days"`
	LeaveDays       int64           `json:"leave_days"`
	UnpaidLeaveDays int64           `json:"unpaid_leave_days"`
}

// Input adalah isian SDM per Pejuang per periode untuk komponen per_unit (kuantitas) dan manual (nominal).
type Input struct {
	Quantity decimal.NullDecimal
	Amount   decimal.NullDecimal
}

// Item adalah satu baris slip. Amount selalu positif; Kind menentukan menambah atau mengurangi.
type Item struct {
	Code       string              `json:"code"`
	Name       string              `json:"name"`
	Kind       string              `json:"kind"`
	Quantity   decimal.NullDecimal `json:"quantity"`
	Rate       decimal.NullDecimal `json:"rate"`
	Amount     decimal.Decimal     `json:"amount"`
	ShowOnSlip bool                `json:"show_on_slip"`
	Manual     bool                `json:"manual"`
}

// Result adalah hasil hitung satu Pejuang.
type Result struct {
	Base       decimal.Decimal `json:"base_salary"`
	Gross      decimal.Decimal `json:"gross"`
	Deductions decimal.Decimal `json:"deductions"`
	Net        decimal.Decimal `json:"net"`
	// NetClamped bernilai true bila potongan melebihi pendapatan dan bersih dibulatkan ke 0.
	NetClamped bool   `json:"net_clamped"`
	Items      []Item `json:"items"`
}

// Calculate menghitung ubar satu Pejuang. perEmployee berisi nilai komponen per_employee, inputs berisi isian
// SDM per periode; keduanya dikunci dengan ID komponen. Semua nominal dibulatkan ke rupiah (tanpa sen).
func Calculate(base decimal.Decimal, att Attendance, comps []Component, perEmployee map[string]decimal.Decimal, inputs map[string]Input, minNetZero bool) Result {
	ordered := append([]Component(nil), comps...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SortOrder < ordered[j].SortOrder })

	base = base.Round(0)
	res := Result{Base: base, Items: []Item{{Code: BaseCode, Name: "Ubar pokok", Kind: KindEarning, Amount: base, ShowOnSlip: true}}}
	gross := base

	add := func(c Component, qty, rate decimal.NullDecimal, amount decimal.Decimal, manual bool) {
		amount = amount.Round(0)
		if !amount.IsPositive() {
			return
		}
		res.Items = append(res.Items, Item{
			Code: c.Code, Name: c.Name, Kind: c.Kind, Quantity: qty, Rate: rate, Amount: amount, ShowOnSlip: c.ShowOnSlip, Manual: manual,
		})
		if c.Kind == KindEarning {
			gross = gross.Add(amount)
		}
	}
	count := func(n int64) decimal.NullDecimal { return decimal.NewNullDecimal(decimal.NewFromInt(n)) }
	perDay := func(c Component, days int64) {
		if days <= 0 {
			return
		}
		if c.Rate.Valid {
			add(c, count(days), c.Rate, c.Rate.Decimal.Mul(decimal.NewFromInt(days)), false)
			return
		}
		divisor := DefaultDivisor
		if c.Divisor.Valid && c.Divisor.Decimal.IsPositive() {
			divisor = c.Divisor.Decimal
		}
		daily := base.Div(divisor)
		add(c, count(days), decimal.NewNullDecimal(daily.Round(0)), daily.Mul(decimal.NewFromInt(days)), false)
	}
	percentOf := func(c Component, of decimal.Decimal) {
		if !c.Percent.Valid {
			return
		}
		add(c, decimal.NullDecimal{}, decimal.NullDecimal{}, of.Mul(c.Percent.Decimal).Div(decimal.NewFromInt(100)), false)
	}

	// Tahap 1: semua komponen kecuali persen dari bruto (yang butuh bruto final).
	for _, c := range ordered {
		rate := c.Rate.Decimal
		switch c.Method {
		case MethodFixed:
			add(c, decimal.NullDecimal{}, decimal.NullDecimal{}, rate, false)
		case MethodPerEmployee:
			if v, ok := perEmployee[c.ID]; ok {
				add(c, decimal.NullDecimal{}, decimal.NullDecimal{}, v, false)
			}
		case MethodPerPresentDay:
			add(c, count(att.PresentDays), c.Rate, rate.Mul(decimal.NewFromInt(att.PresentDays)), false)
		case MethodPerLate:
			add(c, count(att.LateCount), c.Rate, rate.Mul(decimal.NewFromInt(att.LateCount)), false)
		case MethodPerLateMinute:
			add(c, count(att.LateMinutes), c.Rate, rate.Mul(decimal.NewFromInt(att.LateMinutes)), false)
		case MethodPerAbsentDay:
			perDay(c, att.AbsentDays)
		case MethodPerUnpaidLeaveDay:
			perDay(c, att.UnpaidLeaveDays)
		case MethodPercentOfBase:
			percentOf(c, base)
		case MethodPerUnit:
			if in, ok := inputs[c.ID]; ok && in.Quantity.Valid {
				add(c, in.Quantity, c.Rate, rate.Mul(in.Quantity.Decimal), true)
			}
		case MethodManual:
			if in, ok := inputs[c.ID]; ok && in.Amount.Valid {
				add(c, decimal.NullDecimal{}, decimal.NullDecimal{}, in.Amount.Decimal, true)
			}
		}
	}
	// Tahap 2: persen dari bruto. Pendapatan persen-bruto tidak ikut menjadi dasar komponen persen-bruto lain.
	grossBase := gross
	for _, c := range ordered {
		if c.Method == MethodPercentOfGross {
			percentOf(c, grossBase)
		}
	}

	res.Gross = gross
	for _, it := range res.Items {
		if it.Kind == KindDeduction {
			res.Deductions = res.Deductions.Add(it.Amount)
		}
	}
	res.Net = res.Gross.Sub(res.Deductions)
	if minNetZero && res.Net.IsNegative() {
		res.Net = decimal.Zero
		res.NetClamped = true
	}
	return res
}
