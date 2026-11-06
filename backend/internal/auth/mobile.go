package auth

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// mobileSessionTTL adalah umur refresh token mobile (MASTER.md §5.2). Tidak diperpanjang saat refresh,
// jadi Pejuang masuk ulang dengan kata sandi setiap 30 hari (D40).
const mobileSessionTTL = 30 * 24 * time.Hour

// MobileDevice adalah data perangkat yang dikirim aplikasi saat login (F5).
type MobileDevice struct {
	DeviceHash string  `json:"device_hash"`
	Model      *string `json:"model"`
	OSVersion  *string `json:"os_version"`
	FCMToken   *string `json:"fcm_token"`
}

// BoundDevice adalah perangkat yang terikat ke sesi mobile.
type BoundDevice struct {
	ID     uuid.UUID
	Status string
}

// DeviceBinder mendaftarkan atau mengenali perangkat di dalam transaksi pembuatan sesi. Diisi paket
// device agar aturan persetujuan perangkat (D32) hanya ada di satu tempat.
type DeviceBinder func(ctx context.Context, q *store.Queries, employeeID, userID uuid.UUID, ip *netip.Addr, d MobileDevice) (BoundDevice, error)

// SetDeviceBinder wajib dipanggil sebelum login mobile dipakai.
func (s *Service) SetDeviceBinder(b DeviceBinder) { s.bindDevice = b }

// MobileTokens dikirim di body karena aplikasi tidak memakai cookie.
type MobileTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	DeviceID     string `json:"device_id"`
	DeviceStatus string `json:"device_status"`
}

// LoginMobile menerbitkan sesi yang terikat perangkat (F5). Akun ber-MFA tetap wajib memasukkan kode,
// karena token mobile juga bisa membuka route admin sesuai role-nya (D40).
func (s *Service) LoginMobile(ctx context.Context, email, password, mfaCode string, device MobileDevice, ip *netip.Addr, userAgent string, now time.Time) (MobileTokens, error) {
	if s.bindDevice == nil {
		return MobileTokens{}, errors.New("pengikat perangkat untuk login mobile belum dipasang")
	}
	user, err := s.checkPassword(ctx, email, password, ip, now)
	if err != nil {
		return MobileTokens{}, err
	}
	mfa, err := s.q.GetUserMfa(ctx, user.ID)
	if err != nil && !pgconv.IsNoRows(err) {
		return MobileTokens{}, err
	}
	if err == nil && mfa.EnabledAt.Valid {
		if mfaCode == "" {
			return MobileTokens{}, httpx.NewError(http.StatusUnauthorized, "AUTH_MFA_REQUIRED", "Masukkan kode dari aplikasi authenticator.")
		}
		if err := s.checkMFACode(ctx, user.ID, mfaCode, now); err != nil {
			s.auditBestEffort(ctx, audit.Entry{UserID: audit.Actor(user.ID), Action: audit.AuthMFAFailed, Entity: "user", EntityID: user.ID, IP: ip})
			return MobileTokens{}, err
		}
	}

	var out MobileTokens
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		bound, err := s.bindDevice(ctx, q, user.EmployeeID, user.ID, ip, device)
		if err != nil {
			return err
		}
		if err := q.RegisterLoginSuccess(ctx, user.ID); err != nil {
			return err
		}
		refresh, err := NewOpaqueToken()
		if err != nil {
			return err
		}
		ua := pgtype.Text{}
		if userAgent != "" {
			ua = pgtype.Text{String: truncate(userAgent, 256), Valid: true}
		}
		expires := pgconv.Time(now.Add(mobileSessionTTL))
		sessionID, err := q.CreateSession(ctx, store.CreateSessionParams{
			UserID: user.ID, Client: "mobile", DeviceID: uuid.NullUUID{UUID: bound.ID, Valid: true},
			RefreshHash: HashToken(refresh), Ip: ip, UserAgent: ua, IdleExpiresAt: expires, AbsoluteExpiresAt: expires,
		})
		if err != nil {
			return err
		}
		if out, err = s.issueMobile(user.ID, user.EmployeeID, sessionID, bound, string(user.Role), refresh, now); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(user.ID), Action: audit.AuthLoginSuccess, Entity: "session", EntityID: sessionID,
			Detail: map[string]any{"client": "mobile", "device_id": bound.ID.String()}, IP: ip,
		})
	})
	if err != nil {
		return MobileTokens{}, err
	}
	return out, nil
}

// RefreshMobile merotasi refresh token mobile. Perangkat yang sudah dicabut tidak bisa memperpanjang sesi.
func (s *Service) RefreshMobile(ctx context.Context, refresh string, ip *netip.Addr, now time.Time) (MobileTokens, error) {
	sess, next, err := s.rotate(ctx, refresh, "mobile", mobileSessionTTL, ip, now)
	if err != nil {
		return MobileTokens{}, err
	}
	return s.issueMobile(sess.UserID, sess.EmployeeID, sess.ID, BoundDevice{ID: sess.DeviceID.UUID, Status: string(sess.DeviceStatus.DeviceStatus)}, string(sess.Role), next, now)
}

func (s *Service) issueMobile(userID, employeeID, sessionID uuid.UUID, device BoundDevice, role, refresh string, now time.Time) (MobileTokens, error) {
	access, err := s.signer.SignMobile(userID, employeeID, sessionID, device.ID, role, now)
	if err != nil {
		return MobileTokens{}, err
	}
	return MobileTokens{
		AccessToken: access, RefreshToken: refresh, ExpiresIn: int(AccessTTL.Seconds()),
		DeviceID: device.ID.String(), DeviceStatus: device.Status,
	}, nil
}
