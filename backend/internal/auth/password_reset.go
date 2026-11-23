package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
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

// Reset kata sandi lewat email (D30, MASTER.md §5.2.1).
const (
	resetTokenTTL  = time.Hour
	resetSendLimit = 30 * time.Second
)

// Mailer mengirim email akun. Diisi dari paket mail saat SMTP tersedia; tanpa itu email tidak dikirim.
type Mailer interface {
	Enabled(ctx context.Context) bool
	Send(ctx context.Context, to, subject, body string) error
}

// SetMailer memasang pengirim email dan alamat web publik untuk tautan reset (PUBLIC_WEB_URL).
func (s *Service) SetMailer(m Mailer, publicWebURL string) {
	s.mailer = m
	s.publicWebURL = strings.TrimRight(publicWebURL, "/")
}

// sendAsync mengirim email di latar belakang setelah transaksi selesai, agar lama respons tidak
// membocorkan apa pun dan kegagalan SMTP tidak menggagalkan aksi utama. Kegagalan dicatat di audit.
func (s *Service) sendAsync(userID uuid.UUID, to, purpose, subject, body string) {
	if s.mailer == nil || to == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), resetSendLimit)
		defer cancel()
		if err := s.mailer.Send(ctx, to, subject, body); err != nil {
			slog.Warn("email akun belum terkirim", "purpose", purpose, "error", err.Error())
			s.auditBestEffort(ctx, audit.Entry{
				UserID: audit.Actor(userID), Action: audit.EmailDeliveryFailed, Entity: "user", EntityID: userID,
				Detail: map[string]any{"purpose": purpose},
			})
		}
	}()
}

// notifySecurity mengirim pemberitahuan keamanan ke email terdaftar bila SMTP aktif (best-effort).
func (s *Service) notifySecurity(ctx context.Context, userID uuid.UUID, purpose, what string) {
	if s.mailer == nil || !s.mailer.Enabled(ctx) {
		return
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil || !user.Email.Valid {
		return
	}
	body := "Assalamu'alaikum.\n\n" + what + " pada akun HRIS Al Bahjah Anda (" +
		time.Now().In(pgconv.WIB).Format("02-01-2006 15:04") + " WIB).\n\n" +
		"Bila ini bukan Anda, segera ganti kata sandi dan hubungi SDM.\n\nEmail ini dikirim otomatis; mohon tidak dibalas."
	s.sendAsync(userID, user.Email.String, purpose, "Pemberitahuan keamanan akun HRIS Al Bahjah", body)
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ForgotPassword membuat tautan reset dan mengirimnya ke email terdaftar. Hasilnya selalu sama agar
// tidak membocorkan akun mana yang terdaftar atau punya email; pemanggil menjawab 202 apa pun yang terjadi.
func (s *Service) ForgotPassword(ctx context.Context, identifier string, ip *netip.Addr, now time.Time) error {
	if s.mailer == nil || s.publicWebURL == "" || !s.mailer.Enabled(ctx) {
		return nil
	}
	user, err := s.findLoginUser(ctx, identifier)
	if pgconv.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !user.IsActive || !user.Email.Valid || user.Email.String == "" {
		return nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := now.Add(resetTokenTTL)
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.InvalidatePasswordResetTokens(ctx, store.InvalidatePasswordResetTokensParams{Now: pgconv.Time(now), UserID: user.ID}); err != nil {
			return err
		}
		if err := q.CreatePasswordResetToken(ctx, store.CreatePasswordResetTokenParams{
			UserID: user.ID, TokenHash: hashResetToken(token), ExpiresAt: pgconv.Time(expires),
		}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(user.ID), Action: audit.AuthPasswordResetRequested, Entity: "user", EntityID: user.ID, IP: ip,
		})
	})
	if err != nil {
		return err
	}
	link := s.publicWebURL + "/reset-kata-sandi/#token=" + token
	body := "Assalamu'alaikum " + user.FullName + ".\n\n" +
		"Kami menerima permintaan untuk mengatur ulang kata sandi akun HRIS Al Bahjah Anda. Buka tautan berikut untuk membuat kata sandi baru:\n\n" +
		link + "\n\n" +
		"Tautan ini hanya bisa dipakai sekali dan berlaku sampai " + expires.In(pgconv.WIB).Format("02-01-2006 15:04") + " WIB.\n" +
		"Bila Anda tidak meminta ini, abaikan email ini; kata sandi Anda tidak berubah.\n\nEmail ini dikirim otomatis; mohon tidak dibalas."
	s.sendAsync(user.ID, user.Email.String, "password_reset", "Atur ulang kata sandi HRIS Al Bahjah", body)
	return nil
}

func errResetTokenInvalid() error {
	return httpx.NewError(http.StatusBadRequest, "RESET_TOKEN_INVALID",
		"Tautan atur ulang kata sandi sudah tidak berlaku. Silakan minta tautan baru dari halaman Lupa kata sandi.")
}

// ResetPasswordWithToken memakai tautan sekali pakai untuk memasang kata sandi baru, membuka kunci
// akun, dan mencabut semua sesi. MFA tetap diminta saat login berikutnya.
func (s *Service) ResetPasswordWithToken(ctx context.Context, token, password string, ip *netip.Addr, now time.Time) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 {
		return errResetTokenInvalid()
	}
	if password == "" {
		return httpx.Validation(map[string]string{"password": "Kata sandi baru wajib diisi."})
	}
	if len(password) > 256 {
		return httpx.Validation(map[string]string{"password": "Kata sandi melebihi panjang maksimal."})
	}
	// Panjang dan kata sandi umum diperiksa sebelum token dipakai, agar salah isi tidak menghabiskan tautan.
	if msg := ValidatePassword(password, ""); msg != "" {
		return httpx.Validation(map[string]string{"password": msg})
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	var userID uuid.UUID
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		id, err := q.ConsumePasswordResetToken(ctx, store.ConsumePasswordResetTokenParams{Now: pgconv.Time(now), TokenHash: hashResetToken(token)})
		if pgconv.IsNoRows(err) {
			return errResetTokenInvalid()
		}
		if err != nil {
			return err
		}
		user, err := q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		if !user.IsActive {
			return errResetTokenInvalid()
		}
		if msg := ValidatePassword(password, user.Email.String); msg != "" {
			return httpx.Validation(map[string]string{"password": msg})
		}
		userID = id
		if err := q.SetUserPassword(ctx, store.SetUserPasswordParams{ID: id, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.SetUserLock(ctx, store.SetUserLockParams{ID: id}); err != nil {
			return err
		}
		if err := q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: id, Reason: pgtype.Text{String: "password_reset", Valid: true}}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(id), Action: audit.AuthPasswordResetCompleted, Entity: "user", EntityID: id, IP: ip,
		})
	})
	if err != nil {
		return err
	}
	s.notifySecurity(ctx, userID, "password_reset_completed", "Kata sandi baru dipasang lewat tautan atur ulang kata sandi")
	return nil
}
