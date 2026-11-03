// Package audit menulis audit_logs. Nama event hanya dari registry MASTER.md §5.11.
package audit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	AuthLoginSuccess      = "auth.login_success"
	AuthLoginFailed       = "auth.login_failed"
	AuthLogout            = "auth.logout"
	AuthMFAEnabled        = "auth.mfa_enabled"
	AuthMFAFailed         = "auth.mfa_failed"
	AuthPasswordChanged   = "auth.password_changed"
	AuthSessionRevoked    = "auth.session_revoked"
	AuthRefreshReuse      = "auth.refresh_reuse_detected"
	UserCreated           = "user.created"
	UserLocked            = "user.locked"
	UserUnlocked          = "user.unlocked"
	UserPasswordReset     = "user.password_reset"
	OrgUpdated            = "org.updated"
	EmployeeCreated       = "employee.created"
	EmployeeUpdated       = "employee.updated"
	EmployeeDeactivated   = "employee.deactivated"
	EmployeeSalaryChanged = "employee.salary_changed"
	AnnouncementCreated   = "announcement.created"
	AnnouncementUpdated   = "announcement.updated"
	AnnouncementDeleted   = "announcement.deleted"
	SettingsChanged       = "settings.changed"
	UserRoleChanged       = "user.role_changed"
	DeviceRegistered      = "device.registered"
	DeviceApproved        = "device.approved"
	DeviceRevoked         = "device.revoked"
	AttendanceClockIn     = "attendance.clock_in"
	AttendanceClockOut    = "attendance.clock_out"
	AttendanceRejected    = "attendance.rejected"
)

// Entry adalah satu baris audit. Detail tidak boleh memuat kata sandi atau token.
type Entry struct {
	UserID   uuid.NullUUID
	Action   string
	Entity   string
	EntityID uuid.UUID
	Detail   map[string]any
	IP       *netip.Addr
}

// Write memakai q agar bisa ikut transaksi aksi yang dicatat.
func Write(ctx context.Context, q *store.Queries, e Entry) error {
	var detail []byte
	if e.Detail != nil {
		var err error
		if detail, err = json.Marshal(e.Detail); err != nil {
			return err
		}
	}
	entity := pgtype.Text{}
	if e.Entity != "" {
		entity = pgtype.Text{String: e.Entity, Valid: true}
	}
	entityID := uuid.NullUUID{}
	if e.EntityID != uuid.Nil {
		entityID = uuid.NullUUID{UUID: e.EntityID, Valid: true}
	}
	return q.InsertAuditLog(ctx, store.InsertAuditLogParams{
		UserID:   e.UserID,
		Action:   e.Action,
		Entity:   entity,
		EntityID: entityID,
		Detail:   detail,
		Ip:       e.IP,
	})
}

// ClientIP mengambil IP klien dari RemoteAddr. Di balik reverse proxy, nilainya sudah
// diganti middleware realIP di internal/server.
func ClientIP(r *http.Request) *netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return nil
	}
	addr = addr.Unmap().WithZone("")
	return &addr
}

// Actor mengubah user ID menjadi NullUUID untuk Entry.
func Actor(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }
