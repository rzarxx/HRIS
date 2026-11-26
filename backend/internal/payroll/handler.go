package payroll

import (
	"net/http"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
)

func actor(r *http.Request) audit.Entry {
	p, _ := auth.PrincipalFrom(r.Context())
	return audit.Entry{UserID: audit.Actor(p.UserID), IP: audit.ClientIP(r)}
}
