package permission

import "testing"

// Tabel ini menyalin matriks MASTER.md §3.2. Bila salah satu berubah, tes gagal
// sehingga dokumen dan kode harus diperbarui bersama.
func TestMatrix(t *testing.T) {
	cases := []struct {
		key                         string
		pejuang, subdiv, kadiv, sdm Scope
	}{
		{ProfileRead, ScopeSelf, ScopeSelf, ScopeSelf, ScopeSelf},
		{OrgRead, ScopeAll, ScopeAll, ScopeAll, ScopeAll},
		{OrgWrite, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{EmployeeRead, ScopeNone, ScopeSubdivision, ScopeDivision, ScopeAll},
		{EmployeeWrite, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{UserManage, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{DeviceReview, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{FaceReset, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{AttendanceClock, ScopeSelf, ScopeSelf, ScopeSelf, ScopeSelf},
		{AttendanceRead, ScopeSelf, ScopeSubdivision, ScopeDivision, ScopeAll},
		{AttendanceReview, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{AttendanceReport, ScopeNone, ScopeNone, ScopeDivision, ScopeAll},
		{LocationManage, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{LeaveRead, ScopeSelf, ScopeSubdivision, ScopeDivision, ScopeAll},
		{ApprovalAct, ScopeNone, ScopeSelf, ScopeSelf, ScopeSelf},
		{PayslipRead, ScopeSelf, ScopeSelf, ScopeSelf, ScopeSelf},
		{PayrollManage, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{PayrollFinalize, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{WarningManage, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{AnnouncementRead, ScopeAll, ScopeAll, ScopeAll, ScopeAll},
		{AnnouncementManage, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{DashboardRead, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
		{AuditRead, ScopeNone, ScopeNone, ScopeNone, ScopeAll},
	}
	for _, c := range cases {
		for role, want := range map[string]Scope{RolePejuang: c.pejuang, RoleSubdiv: c.subdiv, RoleKadiv: c.kadiv, RoleSDM: c.sdm} {
			if got := ScopeOf(role, c.key); got != want {
				t.Errorf("ScopeOf(%s, %s) = %q, ingin %q", role, c.key, got, want)
			}
		}
	}
}

func TestDenyByDefault(t *testing.T) {
	if Allows("admin", DashboardRead) {
		t.Error("role tak dikenal tidak boleh punya akses")
	}
	if Allows(RoleSDM, "permission:ngarang") {
		t.Error("permission tak dikenal harus ditolak")
	}
	if got := Keys("admin"); len(got) != 0 {
		t.Errorf("Keys role tak dikenal = %v", got)
	}
}

func TestKeysConsistentWithAllows(t *testing.T) {
	for _, role := range []string{RolePejuang, RoleSubdiv, RoleKadiv, RoleSDM} {
		for _, key := range Keys(role) {
			if !Allows(role, key) {
				t.Errorf("%s: Keys memuat %s tetapi Allows false", role, key)
			}
		}
	}
}
