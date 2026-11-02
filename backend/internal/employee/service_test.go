package employee

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var validEmail = "rudi.hartono@pejuang.id"

func validInput() Input {
	return Input{
		FullName:      "Rudi Hartono",
		Email:         &validEmail,
		SubdivisionID: uuid.NewString(),
		BaseSalary:    "4250000.00",
		Status:        "active",
	}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	apiErr, ok := err.(*httpx.Error)
	if !ok || apiErr.Code != "VALIDATION_FAILED" {
		t.Fatalf("ingin VALIDATION_FAILED, dapat %v", err)
	}
	return apiErr.Details["fields"].(map[string]string)
}

func TestValidateAcceptsMinimalInput(t *testing.T) {
	v, err := validInput().validate()
	if err != nil {
		t.Fatal(err)
	}
	if v.BaseSalary.StringFixed(2) != "4250000.00" {
		t.Errorf("gaji = %s", v.BaseSalary)
	}
	if v.Phone.Valid || v.JoinDate.Valid {
		t.Error("field opsional kosong harus NULL")
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		field  string
		mutate func(*Input)
	}{
		{"full_name", func(i *Input) { i.FullName = "  " }},
		{"full_name", func(i *Input) { i.FullName = "Al" }},
		{"email", func(i *Input) { i.Email = str("bukan-email") }},
		{"email", func(i *Input) { i.Email = nil }},
		{"email", func(i *Input) { i.Email = str("  ") }},
		{"phone", func(i *Input) { i.Phone = str("12345") }},
		{"nip", func(i *Input) { i.Nip = str("A!") }},
		{"subdivision_id", func(i *Input) { i.SubdivisionID = "bukan-uuid" }},
		{"join_date", func(i *Input) { i.JoinDate = str("02/11/2026") }},
		{"base_salary", func(i *Input) { i.BaseSalary = "" }},
		{"base_salary", func(i *Input) { i.BaseSalary = "-1" }},
		{"base_salary", func(i *Input) { i.BaseSalary = "abc" }},
		{"base_salary", func(i *Input) { i.BaseSalary = "1.234" }},
		{"base_salary", func(i *Input) { i.BaseSalary = "99999999999" }},
		{"status", func(i *Input) { i.Status = "dipecat" }},
	}
	for _, c := range cases {
		in := validInput()
		c.mutate(&in)
		_, err := in.validate()
		if err == nil {
			t.Errorf("%s: input salah diterima: %+v", c.field, in)
			continue
		}
		if _, ok := fieldsOf(t, err)[c.field]; !ok {
			t.Errorf("%s: pesan error tidak ada di field itu: %v", c.field, err)
		}
	}
}

// base_salary hanya untuk sdm dan pemilik data (MASTER.md §7.6).
func TestSalaryVisibility(t *testing.T) {
	owner := uuid.New()
	row := store.GetEmployeeScopedRow{ID: owner}
	cases := []struct {
		name string
		p    auth.Principal
		want bool
	}{
		{"sdm", auth.Principal{Role: permission.RoleSDM, EmployeeID: uuid.New()}, true},
		{"kadiv lain", auth.Principal{Role: permission.RoleKadiv, EmployeeID: uuid.New()}, false},
		{"subdiv lain", auth.Principal{Role: permission.RoleSubdiv, EmployeeID: uuid.New()}, false},
		{"pemilik data", auth.Principal{Role: permission.RoleSubdiv, EmployeeID: owner}, true},
	}
	for _, c := range cases {
		if got := present(c.p, row).BaseSalary != nil; got != c.want {
			t.Errorf("%s: terlihat=%v", c.name, got)
		}
	}
}
