package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Ganti email lewat konfirmasi (D59, MASTER.md §5.2.1). Email login hanya berubah setelah pemilik
// email baru membuka tautan; pemilik email lama selalu diberi tahu.
const (
	emailChangeTTL       = 24 * time.Hour
	emailChangeLimit     = 3
	emailChangeLimitSpan = time.Hour
)

// EmailConfirmationEnabled menjawab apakah ganti email perlu dikonfirmasi lewat email. Tanpa SMTP
// atau PUBLIC_WEB_URL, tautan tidak bisa dikirim sehingga email berubah langsung setelah cek kata sandi.
func (s *Service) EmailConfirmationEnabled(ctx context.Context) bool {
	return s.mailer != nil && s.publicWebURL != "" && s.mailer.Enabled(ctx)
}

// MaskEmail menyamarkan alamat untuk pemberitahuan ke email lama, contoh si***@gmail.com.
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := email[:at], email[at:]
	keep := 2
	if len(local) <= 2 {
		keep = 1
	}
	return local[:keep] + "***" + domain
}

func errEmailInUse() error {
	return httpx.Duplicate("email", "Email sudah dipakai Pejuang lain.")
}

func errEmailChangeInvalid() error {
	return httpx.NewError(http.StatusBadRequest, "EMAIL_CHANGE_INVALID",
		"Tautan konfirmasi email sudah tidak berlaku. Silakan ajukan ganti email lagi dari Pengaturan atau aplikasi.")
}

// RequestEmailChange mencatat permintaan ganti email milik sendiri, mengirim tautan konfirmasi ke
// email baru, dan memberi tahu email lama. Email login belum berubah sampai tautan dibuka.
func (s *Service) RequestEmailChange(ctx context.Context, userID, employeeID uuid.UUID, oldEmail, newEmail string, ip *netip.Addr, now time.Time) error {
	inUse, err := s.q.EmailInUse(ctx, store.EmailInUseParams{Email: newEmail, EmployeeID: employeeID})
	if err != nil {
		return err
	}
	if inUse {
		return errEmailInUse()
	}
	recent, err := s.q.CountEmailChangeRequestsSince(ctx, store.CountEmailChangeRequestsSinceParams{
		UserID: userID, Since: pgconv.Time(now.Add(-emailChangeLimitSpan)),
	})
	if err != nil {
		return err
	}
	if recent >= emailChangeLimit {
		return httpx.NewError(http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan ganti email. Coba lagi dalam satu jam.")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := now.Add(emailChangeTTL)
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.InvalidateEmailChangeRequests(ctx, store.InvalidateEmailChangeRequestsParams{Now: pgconv.Time(now), UserID: userID}); err != nil {
			return err
		}
		if err := q.CreateEmailChangeRequest(ctx, store.CreateEmailChangeRequestParams{
			UserID: userID, NewEmail: newEmail, TokenHash: hashResetToken(token), ExpiresAt: pgconv.Time(expires),
		}); err != nil {
			return err
		}
		// Hanya penanda, bukan alamatnya, agar audit tidak menyimpan email baru yang belum terbukti.
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(userID), Action: audit.UserEmailChangeRequested, Entity: "user", EntityID: userID, IP: ip,
		})
	})
	if err != nil {
		return err
	}
	link := s.publicWebURL + "/konfirmasi-email/#token=" + token
	s.sendAsync(userID, newEmail, "email_change_confirm", "Konfirmasi email baru HRIS Al Bahjah",
		"Assalamu'alaikum.\n\nAlamat ini diajukan sebagai email baru akun HRIS Al Bahjah. Buka tautan berikut untuk mengonfirmasi:\n\n"+
			link+"\n\nTautan berlaku sampai "+expires.In(pgconv.WIB).Format("02-01-2006 15:04")+" WIB dan hanya bisa dipakai sekali.\n"+
			"Bila Anda tidak mengajukan ini, abaikan email ini.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.")
	if oldEmail != "" {
		s.sendAsync(userID, oldEmail, "email_change_requested", "Permintaan ganti email akun HRIS Al Bahjah",
			"Assalamu'alaikum.\n\nAda permintaan untuk mengganti email akun HRIS Al Bahjah Anda menjadi "+MaskEmail(newEmail)+
				". Email login belum berubah sampai email baru dikonfirmasi.\n\n"+
				"Bila ini bukan Anda, segera ganti kata sandi dan hubungi SDM.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.")
	}
	return nil
}

// PendingEmail mengembalikan email baru yang masih menunggu konfirmasi, bila ada.
func (s *Service) PendingEmail(ctx context.Context, userID uuid.UUID, now time.Time) (*string, error) {
	v, err := s.q.GetPendingEmailChange(ctx, store.GetPendingEmailChangeParams{UserID: userID, Now: pgconv.Time(now)})
	if pgconv.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// CancelEmailChange membatalkan permintaan ganti email yang belum dikonfirmasi.
func (s *Service) CancelEmailChange(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return s.q.InvalidateEmailChangeRequests(ctx, store.InvalidateEmailChangeRequestsParams{Now: pgconv.Time(now), UserID: userID})
}

// ConfirmEmailChange memakai tautan konfirmasi: email Pejuang dan email login berubah dalam satu
// transaksi, semua sesi dicabut, dan email lama diberi tahu.
func (s *Service) ConfirmEmailChange(ctx context.Context, token string, ip *netip.Addr, now time.Time) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 {
		return errEmailChangeInvalid()
	}
	var userID uuid.UUID
	var oldEmail, newEmail string
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.ConsumeEmailChangeRequest(ctx, store.ConsumeEmailChangeRequestParams{Now: pgconv.Time(now), TokenHash: hashResetToken(token)})
		if pgconv.IsNoRows(err) {
			return errEmailChangeInvalid()
		}
		if err != nil {
			return err
		}
		user, err := q.GetUserByID(ctx, row.UserID)
		if err != nil {
			return err
		}
		if !user.IsActive {
			return errEmailChangeInvalid()
		}
		inUse, err := q.EmailInUse(ctx, store.EmailInUseParams{Email: row.NewEmail, EmployeeID: user.EmployeeID})
		if err != nil {
			return err
		}
		if inUse {
			return errEmailInUse()
		}
		email := pgtype.Text{String: row.NewEmail, Valid: true}
		if _, err := q.SetEmployeeEmail(ctx, store.SetEmployeeEmailParams{ID: user.EmployeeID, Email: email}); err != nil {
			return mapEmailUnique(err)
		}
		if err := q.UpdateUserEmail(ctx, store.UpdateUserEmailParams{EmployeeID: user.EmployeeID, Email: email}); err != nil {
			return mapEmailUnique(err)
		}
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: user.ID, Reason: pgtype.Text{String: "email_changed", Valid: true}}); err != nil {
			return err
		}
		userID, oldEmail, newEmail = user.ID, user.Email.String, row.NewEmail
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(user.ID), Action: audit.UserEmailChanged, Entity: "user", EntityID: user.ID, IP: ip,
			Detail: map[string]any{"source": "self"},
		})
	})
	if err != nil {
		return err
	}
	if oldEmail != "" && !strings.EqualFold(oldEmail, newEmail) {
		s.sendAsync(userID, oldEmail, "email_changed", "Email akun HRIS Al Bahjah diganti",
			"Assalamu'alaikum.\n\nEmail login akun HRIS Al Bahjah Anda sudah diganti menjadi "+MaskEmail(newEmail)+
				". Semua sesi lama sudah dikeluarkan.\n\nBila ini bukan Anda, segera hubungi SDM.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.")
	}
	return nil
}

// NotifyEmailChanged memberi tahu email lama dan baru setelah email login berubah langsung: oleh SDM
// (byAdmin), atau oleh Pejuang sendiri saat konfirmasi lewat email tidak dipakai (best-effort,
// setelah transaksi selesai).
func (s *Service) NotifyEmailChanged(ctx context.Context, employeeID uuid.UUID, oldEmail, newEmail string, byAdmin bool) {
	if s.mailer == nil || !s.mailer.Enabled(ctx) || strings.EqualFold(oldEmail, newEmail) {
		return
	}
	user, err := s.q.GetUserByEmployee(ctx, employeeID)
	if err != nil {
		return
	}
	who, purpose := "Email login akun HRIS Al Bahjah Anda diganti", "email_changed"
	if byAdmin {
		who, purpose = "SDM mengganti email login akun HRIS Al Bahjah Anda", "email_changed_by_admin"
	}
	if oldEmail != "" {
		target := "dikosongkan"
		if newEmail != "" {
			target = "menjadi " + MaskEmail(newEmail)
		}
		s.sendAsync(user.ID, oldEmail, purpose, "Email akun HRIS Al Bahjah diganti",
			"Assalamu'alaikum.\n\n"+who+" ("+target+").\n\nBila Anda tidak meminta perubahan ini, segera hubungi SDM.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.")
	}
	if newEmail != "" && byAdmin {
		s.sendAsync(user.ID, newEmail, purpose, "Email ini sekarang dipakai untuk akun HRIS Al Bahjah",
			"Assalamu'alaikum.\n\nSDM memasang alamat ini sebagai email login akun HRIS Al Bahjah Anda. Anda bisa masuk dengan email ini dan memakai Lupa kata sandi bila perlu.\n\n"+
				"Bila Anda tidak merasa memiliki akun HRIS Al Bahjah, hubungi SDM.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.")
	}
}

func mapEmailUnique(err error) error {
	if constraint, ok := pgconv.UniqueConstraint(err); ok && strings.Contains(constraint, "email") {
		return errEmailInUse()
	}
	return err
}
