package employee

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var today = time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)

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
	v, err := validInput().validate(today)
	if err != nil {
		t.Fatal(err)
	}
	if v.BaseSalary.StringFixed(2) != "4250000.00" {
		t.Errorf("ubar = %s", v.BaseSalary)
	}
	if v.Phone.Valid || v.JoinDate.Valid {
		t.Error("field opsional kosong harus NULL")
	}
}

// D53: email opsional karena login bisa memakai NIPY atau nomor WA.
func TestValidateEmailOptional(t *testing.T) {
	for _, email := range []*string{nil, new(string)} {
		in := validInput()
		in.Email = email
		v, err := in.validate(today)
		if err != nil {
			t.Fatal(err)
		}
		if email == nil && v.Email.Valid {
			t.Error("email nil harus NULL")
		}
	}
}

func TestValidateNormalizesPhoneAndChildren(t *testing.T) {
	str := func(s string) *string { return &s }
	yes, count := true, 3
	in := validInput()
	in.Phone = str("0812-3456-7890")
	in.Gender = str("P")
	in.MaritalStatus = str("menikah")
	in.HasChildren, in.ChildrenCount = &yes, &count
	in.Children = &[]Child{
		{ChildOrder: 1, FullName: " Aisyah ", Gender: str("P"), BirthDate: str("2015-04-02"), Relation: str("kandung"), Education: str("sd_mi"), SchoolName: str("MI Al Bahjah")},
		{ChildOrder: 2, FullName: "Umar", Education: str("pondok_pesantren")},
	}
	v, err := in.validate(today)
	if err != nil {
		t.Fatal(err)
	}
	if v.Phone.String != "6281234567890" {
		t.Errorf("phone = %q", v.Phone.String)
	}
	if !v.SetChildren || len(v.Children) != 2 || v.Children[0].FullName != "Aisyah" || !v.Children[0].BirthDate.Valid {
		t.Errorf("anak = %+v", v.Children)
	}
	// Tanpa field children, data anak lama tidak disentuh.
	in.Children = nil
	if v, _ := in.validate(today); v.SetChildren {
		t.Error("children nil tidak boleh mengganti data anak")
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
		{"gender", func(i *Input) { i.Gender = str("X") }},
		{"marital_status", func(i *Input) { i.MaritalStatus = str("kawin") }},
		{"children_count", func(i *Input) { n := -1; i.ChildrenCount = &n }},
		{"has_children", func(i *Input) { no, n := false, 2; i.HasChildren, i.ChildrenCount = &no, &n }},
		{"children_count", func(i *Input) {
			n := 0
			i.ChildrenCount = &n
			i.Children = &[]Child{{ChildOrder: 1, FullName: "Aisyah"}}
		}},
		{"children.0.full_name", func(i *Input) { i.Children = &[]Child{{ChildOrder: 1, FullName: "A"}} }},
		{"children.1.child_order", func(i *Input) {
			i.Children = &[]Child{{ChildOrder: 1, FullName: "Aisyah"}, {ChildOrder: 1, FullName: "Fatimah"}}
		}},
		{"children.0.relation", func(i *Input) { i.Children = &[]Child{{ChildOrder: 1, FullName: "Aisyah", Relation: str("tetangga")}} }},
		{"children.0.education", func(i *Input) { i.Children = &[]Child{{ChildOrder: 1, FullName: "Aisyah", Education: str("s3")}} }},
		{"children.0.birth_date", func(i *Input) {
			i.Children = &[]Child{{ChildOrder: 1, FullName: "Aisyah", BirthDate: str("2027-01-01")}}
		}},
	}
	for _, c := range cases {
		in := validInput()
		c.mutate(&in)
		_, err := in.validate(today)
		if err == nil {
			t.Errorf("%s: input salah diterima: %+v", c.field, in)
			continue
		}
		if _, ok := fieldsOf(t, err)[c.field]; !ok {
			t.Errorf("%s: pesan error tidak ada di field itu: %v", c.field, err)
		}
	}
}

// Data anak hanya untuk SDM, super admin, dan Pejuang sendiri; atasan tidak (D53).
func TestChildrenVisibility(t *testing.T) {
	owner := uuid.New()
	row := store.GetEmployeeScopedRow{ID: owner, ChildrenCount: pgtype.Int2{Int16: 2, Valid: true}}
	cases := []struct {
		name string
		p    auth.Principal
		want bool
	}{
		{"sdm", auth.Principal{Role: permission.RoleSDM, EmployeeID: uuid.New()}, true},
		{"super admin", auth.Principal{Role: permission.RoleSuperadmin, EmployeeID: uuid.New()}, true},
		{"kadiv", auth.Principal{Role: permission.RoleKadiv, EmployeeID: uuid.New()}, false},
		{"subdiv", auth.Principal{Role: permission.RoleSubdiv, EmployeeID: uuid.New()}, false},
		{"pemilik data", auth.Principal{Role: permission.RolePejuang, EmployeeID: owner}, true},
	}
	for _, c := range cases {
		if got := present(c.p, row).ChildrenCount != nil; got != c.want {
			t.Errorf("%s: terlihat=%v", c.name, got)
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
