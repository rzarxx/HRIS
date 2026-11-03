// Package server merakit router. Setiap route /v1 terdaftar di tabel routes beserta
// permission-nya; route tanpa permission eksplisit hanya boleh bertanda Public/AnyUser.
package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rzarxx/HRIS/backend/internal/announcement"
	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/dashboard"
	"github.com/rzarxx/HRIS/backend/internal/device"
	"github.com/rzarxx/HRIS/backend/internal/employee"
	"github.com/rzarxx/HRIS/backend/internal/location"
	"github.com/rzarxx/HRIS/backend/internal/org"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/report"
	"github.com/rzarxx/HRIS/backend/internal/shift"
)

// Deps adalah handler tiap modul.
type Deps struct {
	Auth         *auth.Handler
	Org          *org.Handler
	Employee     *employee.Handler
	Announcement *announcement.Handler
	Dashboard    *dashboard.Handler
	Report       *report.Handler
	Location     *location.Handler
	Device       *device.Handler
	Attendance   *attendance.Handler
	Shift        *shift.Handler
	Signer       *auth.TokenSigner
	SessionCheck auth.SessionChecker
	Limiter      auth.Limiter
	Ready        func(context.Context) error
	Options      Options
}

// Access menyatakan siapa yang boleh memanggil route.
type Access int

const (
	Public  Access = iota // tanpa login
	AnyUser               // login saja, data milik sendiri
	Guarded               // login + permission
)

// Route adalah satu baris tabel endpoint.
type Route struct {
	Method      string
	Path        string
	Access      Access
	Permissions []string
	Handler     httpx.Handler
}

func routes(d Deps) []Route {
	guarded := func(method, path string, h httpx.Handler, perms ...string) Route {
		return Route{Method: method, Path: path, Access: Guarded, Permissions: perms, Handler: h}
	}
	return []Route{
		{http.MethodPost, "/v1/auth/web/login", Public, nil, d.Auth.Login},
		{http.MethodPost, "/v1/auth/web/refresh", Public, nil, d.Auth.Refresh},
		{http.MethodPost, "/v1/auth/web/mfa/verify", Public, nil, d.Auth.MFAVerify},
		{http.MethodPost, "/v1/auth/activate", Public, nil, d.Auth.Activate},
		{http.MethodPost, "/v1/auth/logout", AnyUser, nil, d.Auth.Logout},
		{http.MethodPost, "/v1/auth/password/change", AnyUser, nil, d.Auth.ChangePassword},
		{http.MethodGet, "/v1/auth/sessions", AnyUser, nil, d.Auth.ListSessions},
		{http.MethodDelete, "/v1/auth/sessions/{session_id}", AnyUser, nil, d.Auth.RevokeSession},
		{http.MethodPost, "/v1/auth/mfa/setup", AnyUser, nil, d.Auth.MFASetup},
		{http.MethodPost, "/v1/auth/mfa/enable", AnyUser, nil, d.Auth.MFAEnable},
		// Tetap terbuka saat sandi/MFA belum dipenuhi agar web tahu apa yang harus dilakukan.
		{http.MethodGet, "/v1/me", AnyUser, nil, d.Auth.GetMe},

		guarded(http.MethodPost, "/v1/admin/users/{user_id}/reset-password", d.Auth.AdminResetPassword, permission.UserReset),
		guarded(http.MethodPost, "/v1/admin/users/{user_id}/lock", d.Auth.AdminLock, permission.UserManage),
		guarded(http.MethodPost, "/v1/admin/users/{user_id}/unlock", d.Auth.AdminUnlock, permission.UserManage),
		guarded(http.MethodPost, "/v1/admin/users/{user_id}/revoke-sessions", d.Auth.AdminRevokeSessions, permission.UserManage),

		guarded(http.MethodGet, "/v1/admin/divisions", d.Org.ListDivisions, permission.OrgWrite),
		guarded(http.MethodPost, "/v1/admin/divisions", d.Org.CreateDivision, permission.OrgWrite),
		guarded(http.MethodPatch, "/v1/admin/divisions/{id}", d.Org.UpdateDivision, permission.OrgWrite),
		guarded(http.MethodDelete, "/v1/admin/divisions/{id}", d.Org.DeleteDivision, permission.OrgWrite),

		guarded(http.MethodGet, "/v1/admin/subdivisions", d.Org.ListSubdivisions, permission.OrgWrite),
		guarded(http.MethodPost, "/v1/admin/subdivisions", d.Org.CreateSubdivision, permission.OrgWrite),
		guarded(http.MethodPatch, "/v1/admin/subdivisions/{id}", d.Org.UpdateSubdivision, permission.OrgWrite),
		guarded(http.MethodDelete, "/v1/admin/subdivisions/{id}", d.Org.DeleteSubdivision, permission.OrgWrite),

		guarded(http.MethodGet, "/v1/admin/employees", d.Employee.List, permission.EmployeeRead),
		guarded(http.MethodPost, "/v1/admin/employees", d.Employee.Create, permission.EmployeeWrite),
		guarded(http.MethodGet, "/v1/admin/employees/{id}", d.Employee.Get, permission.EmployeeRead),
		guarded(http.MethodPatch, "/v1/admin/employees/{id}", d.Employee.Update, permission.EmployeeWrite),
		guarded(http.MethodPost, "/v1/admin/employees/{id}/deactivate", d.Employee.Deactivate, permission.EmployeeWrite),

		guarded(http.MethodGet, "/v1/admin/employees/{id}/locations", d.Location.ForEmployee, permission.EmployeeWrite),
		guarded(http.MethodPut, "/v1/admin/employees/{id}/locations", d.Location.SetForEmployee, permission.EmployeeWrite),

		guarded(http.MethodPost, "/v1/attendance/challenge", d.Attendance.Challenge, permission.AttendanceClock),
		guarded(http.MethodPost, "/v1/attendance/clock-in", d.Attendance.ClockIn, permission.AttendanceClock),
		guarded(http.MethodPost, "/v1/attendance/clock-out", d.Attendance.ClockOut, permission.AttendanceClock),
		guarded(http.MethodGet, "/v1/attendance", d.Attendance.Mine, permission.AttendanceRead),

		guarded(http.MethodPost, "/v1/devices/register", d.Device.Register, permission.DeviceRegister),
		guarded(http.MethodGet, "/v1/devices/me", d.Device.Mine, permission.DeviceRegister),
		guarded(http.MethodGet, "/v1/admin/devices", d.Device.List, permission.DeviceReview),
		guarded(http.MethodPost, "/v1/admin/devices/{id}/approve", d.Device.Approve, permission.DeviceReview),
		guarded(http.MethodPost, "/v1/admin/devices/{id}/revoke", d.Device.Revoke, permission.DeviceReview),

		guarded(http.MethodGet, "/v1/admin/office-locations", d.Location.List, permission.LocationManage),
		guarded(http.MethodPost, "/v1/admin/office-locations", d.Location.Create, permission.LocationManage),
		guarded(http.MethodPatch, "/v1/admin/office-locations/{id}", d.Location.Update, permission.LocationManage),
		guarded(http.MethodDelete, "/v1/admin/office-locations/{id}", d.Location.Delete, permission.LocationManage),

		guarded(http.MethodGet, "/v1/admin/shifts", d.Shift.List, permission.ShiftManage),
		guarded(http.MethodPost, "/v1/admin/shifts", d.Shift.Create, permission.ShiftManage),
		guarded(http.MethodPatch, "/v1/admin/shifts/{id}", d.Shift.Update, permission.ShiftManage),
		guarded(http.MethodDelete, "/v1/admin/shifts/{id}", d.Shift.Delete, permission.ShiftManage),
		guarded(http.MethodGet, "/v1/admin/holidays", d.Shift.ListHolidays, permission.ShiftManage),
		guarded(http.MethodPost, "/v1/admin/holidays", d.Shift.CreateHoliday, permission.ShiftManage),
		guarded(http.MethodDelete, "/v1/admin/holidays/{date}", d.Shift.DeleteHoliday, permission.ShiftManage),

		guarded(http.MethodGet, "/v1/admin/announcements", d.Announcement.List, permission.AnnouncementManage),
		guarded(http.MethodPost, "/v1/admin/announcements", d.Announcement.Create, permission.AnnouncementManage),
		guarded(http.MethodGet, "/v1/admin/announcements/{id}", d.Announcement.Get, permission.AnnouncementManage),
		guarded(http.MethodPatch, "/v1/admin/announcements/{id}", d.Announcement.Update, permission.AnnouncementManage),
		guarded(http.MethodDelete, "/v1/admin/announcements/{id}", d.Announcement.Delete, permission.AnnouncementManage),

		guarded(http.MethodGet, "/v1/admin/dashboard/summary", d.Dashboard.Summary, permission.DashboardRead),
		guarded(http.MethodGet, "/v1/admin/dashboard/org-chart", d.Dashboard.OrgChart, permission.DashboardRead),

		guarded(http.MethodGet, "/v1/admin/reports/attendance", d.Report.Attendance, permission.AttendanceReport),
		// Memuat gaji: base_salary hanya untuk sdm (§7.6), jadi perlu payroll:manage juga.
		guarded(http.MethodGet, "/v1/admin/reports/payroll-preview", d.Report.PayrollPreview, permission.AttendanceReport, permission.PayrollManage),
	}
}

// New membangun http.Handler lengkap dengan urutan middleware MASTER.md §11.2.
func New(d Deps) http.Handler {
	return build(d, routes(d))
}

func build(d Deps, table []Route) http.Handler {
	r := chi.NewRouter()
	r.Use(realIP(d.Options.TrustProxy), requestID, recoverer, requestLogger, limitBody, withTimeout, cors(d.Options.AllowedOrigins), securityHeaders(d.Options.Secure), limitByIP(d.Limiter))

	r.NotFound(func(w http.ResponseWriter, req *http.Request) { httpx.WriteError(w, req, httpx.ErrNotFound()) })
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, httpx.NewError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode tidak diizinkan."))
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if d.Ready != nil && d.Ready(req.Context()) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	authenticate := auth.Authenticate(d.Signer, d.SessionCheck)
	for _, route := range table {
		var chain http.Handler = httpx.H(route.Handler)
		if route.Access == Guarded {
			chain = auth.Require(route.Permissions...)(chain)
		}
		if route.Access != Public {
			if clockPaths[route.Path] {
				chain = limitClock(d.Limiter)(chain)
			}
			chain = authenticate(limitByUser(d.Limiter)(chain))
		}
		r.Method(route.Method, route.Path, chain)
	}
	return r
}
