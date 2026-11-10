// Package permission adalah satu-satunya peta role ke permission (MASTER.md §3.2).
// Permission yang tidak tertulis di sini berarti ditolak.
package permission

import "sort"

type Scope string

const (
	ScopeNone        Scope = ""
	ScopeSelf        Scope = "self"
	ScopeSubdivision Scope = "subdivision"
	ScopeDivision    Scope = "division"
	ScopeAll         Scope = "all"
)

const (
	RolePejuang    = "pejuang"
	RoleSubdiv     = "subdiv"
	RoleKadiv      = "kadiv"
	RoleSDM        = "sdm"
	RoleSuperadmin = "superadmin"
)

// Permission key.
const (
	ProfileRead        = "profile:read"
	ProfileUpdate      = "profile:update"
	OrgRead            = "org:read"
	OrgWrite           = "org:write"
	EmployeeRead       = "employee:read"
	EmployeeWrite      = "employee:write"
	UserManage         = "user:manage"
	UserReset          = "user:reset"
	DeviceRegister     = "device:register"
	DeviceReview       = "device:review"
	FaceEnroll         = "face:enroll"
	FaceReset          = "face:reset"
	AttendanceClock    = "attendance:clock"
	AttendanceRead     = "attendance:read"
	AttendanceReview   = "attendance:review"
	AttendanceReport   = "attendance:report"
	LocationManage     = "location:manage"
	ShiftManage        = "shift:manage"
	LeaveCreate        = "leave:create"
	CashAdvanceCreate  = "cash_advance:create"
	TransferCreate     = "transfer:create"
	LeaveRead          = "leave:read"
	CashAdvanceRead    = "cash_advance:read"
	TransferRead       = "transfer:read"
	ApprovalAct        = "approval:act"
	PayslipRead        = "payslip:read"
	PayrollManage      = "payroll:manage"
	PayrollFinalize    = "payroll:finalize"
	WarningRead        = "warning:read"
	WarningManage      = "warning:manage"
	AnnouncementRead   = "announcement:read"
	AnnouncementManage = "announcement:manage"
	DashboardRead      = "dashboard:read"
	ApprovalFlowManage = "approval_flow:manage"
	AuditRead          = "audit:read"
)

// RequiresMFA menjawab role mana yang wajib memakai MFA (§5.2, D25).
func RequiresMFA(role string) bool { return role == RoleSDM || role == RoleSuperadmin }

// ManagesSalary menjawab siapa yang boleh melihat dan mengubah ubar orang lain (D34).
func ManagesSalary(role string) bool { return role == RoleSDM }

// Permission yang dimiliki semua role, dengan scope-nya.
var base = map[string]Scope{
	ProfileRead:       ScopeSelf,
	ProfileUpdate:     ScopeSelf,
	OrgRead:           ScopeAll,
	DeviceRegister:    ScopeSelf,
	FaceEnroll:        ScopeSelf,
	AttendanceClock:   ScopeSelf,
	AttendanceRead:    ScopeSelf,
	LeaveCreate:       ScopeSelf,
	CashAdvanceCreate: ScopeSelf,
	TransferCreate:    ScopeSelf,
	LeaveRead:         ScopeSelf,
	CashAdvanceRead:   ScopeSelf,
	TransferRead:      ScopeSelf,
	PayslipRead:       ScopeSelf,
	WarningRead:       ScopeSelf,
	AnnouncementRead:  ScopeAll,
}

// Tambahan per role; menimpa scope dari base.
var extra = map[string]map[string]Scope{
	RolePejuang: {},
	// Role tertinggi: semua akses kecuali keuangan (D34). Tidak ada payroll dan pinjaman orang
	// lain, dan ubar orang lain tidak pernah tampil atau bisa diubah (lihat ManagesSalary).
	RoleSuperadmin: {
		OrgWrite:           ScopeAll,
		EmployeeRead:       ScopeAll,
		EmployeeWrite:      ScopeAll,
		UserManage:         ScopeAll,
		UserReset:          ScopeAll,
		DeviceReview:       ScopeAll,
		FaceReset:          ScopeAll,
		AttendanceRead:     ScopeAll,
		AttendanceReview:   ScopeAll,
		AttendanceReport:   ScopeAll,
		LocationManage:     ScopeAll,
		ShiftManage:        ScopeAll,
		LeaveRead:          ScopeAll,
		TransferRead:       ScopeAll,
		ApprovalAct:        ScopeSelf,
		WarningManage:      ScopeAll,
		AnnouncementManage: ScopeAll,
		DashboardRead:      ScopeAll,
		ApprovalFlowManage: ScopeAll,
		AuditRead:          ScopeAll,
	},
	RoleSubdiv: {
		EmployeeRead:    ScopeSubdivision,
		AttendanceRead:  ScopeSubdivision,
		LeaveRead:       ScopeSubdivision,
		CashAdvanceRead: ScopeSubdivision,
		TransferRead:    ScopeSubdivision,
		ApprovalAct:     ScopeSelf, // hanya langkah miliknya
	},
	RoleKadiv: {
		EmployeeRead:     ScopeDivision,
		AttendanceRead:   ScopeDivision,
		AttendanceReport: ScopeDivision,
		LeaveRead:        ScopeDivision,
		CashAdvanceRead:  ScopeDivision,
		TransferRead:     ScopeDivision,
		ApprovalAct:      ScopeSelf,
	},
	RoleSDM: {
		OrgWrite:           ScopeAll,
		EmployeeRead:       ScopeAll,
		EmployeeWrite:      ScopeAll,
		UserManage:         ScopeAll,
		UserReset:          ScopeAll,
		DeviceReview:       ScopeAll,
		FaceReset:          ScopeAll,
		AttendanceRead:     ScopeAll,
		AttendanceReview:   ScopeAll,
		AttendanceReport:   ScopeAll,
		LocationManage:     ScopeAll,
		ShiftManage:        ScopeAll,
		LeaveRead:          ScopeAll,
		CashAdvanceRead:    ScopeAll,
		TransferRead:       ScopeAll,
		ApprovalAct:        ScopeSelf,
		PayrollManage:      ScopeAll,
		PayrollFinalize:    ScopeAll,
		WarningManage:      ScopeAll,
		AnnouncementManage: ScopeAll,
		DashboardRead:      ScopeAll,
		ApprovalFlowManage: ScopeAll,
		AuditRead:          ScopeAll,
	},
}

// ScopeOf mengembalikan scope role untuk key; ScopeNone berarti tidak punya akses.
func ScopeOf(role, key string) Scope {
	perms, ok := extra[role]
	if !ok {
		return ScopeNone // role tak dikenal ditolak
	}
	if s, ok := perms[key]; ok {
		return s
	}
	return base[key]
}

// Allows menjawab apakah role memiliki key.
func Allows(role, key string) bool { return ScopeOf(role, key) != ScopeNone }

// Keys mengembalikan daftar permission role untuk GET /v1/me, terurut.
func Keys(role string) []string {
	perms, ok := extra[role]
	if !ok {
		return []string{}
	}
	keys := make([]string, 0, len(base)+len(perms))
	for k := range base {
		if _, overridden := perms[k]; !overridden {
			keys = append(keys, k)
		}
	}
	for k := range perms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
