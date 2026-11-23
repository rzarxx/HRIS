package employee

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// EmailChanges adalah bagian auth.Service untuk ganti email (D59): konfirmasi lewat email untuk
// perubahan oleh Pejuang sendiri, dan pemberitahuan untuk perubahan langsung.
type EmailChanges interface {
	EmailConfirmationEnabled(ctx context.Context) bool
	RequestEmailChange(ctx context.Context, userID, employeeID uuid.UUID, oldEmail, newEmail string, ip *netip.Addr, now time.Time) error
	NotifyEmailChanged(ctx context.Context, employeeID uuid.UUID, oldEmail, newEmail string, byAdmin bool)
}

// WithEmailChanges memasang konfirmasi dan pemberitahuan ganti email.
func (s *Service) WithEmailChanges(e EmailChanges) *Service {
	s.emails = e
	return s
}

// recordEmailChange mencatat email login yang berubah langsung dan mencabut semua sesi pemiliknya, agar
// perangkat yang masuk dengan email lama ikut keluar (MASTER.md §5.2.1).
func recordEmailChange(ctx context.Context, q *store.Queries, ip audit.Entry, employeeID uuid.UUID, source string) error {
	user, err := q.GetUserByEmployee(ctx, employeeID)
	if err != nil {
		return err
	}
	if source == "admin" {
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "email_changed", Valid: true}}); err != nil {
			return err
		}
	}
	entry := ip
	entry.Action, entry.Entity, entry.EntityID = audit.UserEmailChanged, "user", user.ID
	entry.Detail = map[string]any{"source": source}
	return audit.Write(ctx, q, entry)
}

// adminEmailChanged dipakai ubah data oleh SDM dan impor massal.
func adminEmailChanged(ctx context.Context, q *store.Queries, ip audit.Entry, employeeID uuid.UUID) error {
	return recordEmailChange(ctx, q, ip, employeeID, "admin")
}
