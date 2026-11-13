// Package device mengelola device binding (MASTER.md §5.3, alur F5 dan F16).
package device

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	maxModelLen          = 100
	maxVersionLen        = 50
	maxFCMTokenLen       = 4096
	maxPublicKeyB64Len   = 400
	maxTierReasonLen     = 250
	minTierReasonLen     = 3
	maxManualApprovalDay = 90

	reasonAdminRevoked = "admin_revoked"
	reasonReplaced     = "replaced"
)

// device_hash = SHA-256 heksadesimal dari ID instalasi acak, bukan IMEI (§5.3).
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Device adalah perangkat milik sendiri. Hash, kunci publik mentah, dan token FCM tidak pernah dikembalikan.
type Device struct {
	ID                  string  `json:"id"`
	Model               *string `json:"model"`
	OSVersion           *string `json:"os_version"`
	Status              string  `json:"status"`
	IntegrityTier       string  `json:"integrity_tier"`
	ManualApprovedUntil *string `json:"manual_approved_until"`
	KeyBound            bool    `json:"key_bound"`
	CreatedAt           string  `json:"created_at"`
	LastSeenAt          *string `json:"last_seen_at"`
}

// AdminDevice menambah data Pejuang untuk halaman persetujuan.
type AdminDevice struct {
	Device
	EmployeeID      string  `json:"employee_id"`
	EmployeeName    string  `json:"employee_name"`
	Nip             string  `json:"nip"`
	SubdivisionName string  `json:"subdivision_name"`
	DivisionName    string  `json:"division_name"`
	ApprovedAt      *string `json:"approved_at"`
	ApprovedByName  *string `json:"approved_by_name"`
	RevokedAt       *string `json:"revoked_at"`
	RevokedReason   *string `json:"revoked_reason"`
	// Web memakainya untuk menyembunyikan tombol cabut dari SDM (D34).
	OwnerIsSuperadmin bool `json:"owner_is_superadmin"`
}

// RegisterInput adalah data perangkat dari aplikasi.
type RegisterInput struct {
	DeviceHash string  `json:"device_hash"`
	Model      *string `json:"model"`
	OSVersion  *string `json:"os_version"`
	FCMToken   *string `json:"fcm_token"`
	PublicKey  *string `json:"public_key"`
}

// TierInput mengatur jalur verifikasi integritas perangkat (MASTER.md §5.4.1).
type TierInput struct {
	IntegrityTier       string  `json:"integrity_tier"`
	ManualApprovedUntil *string `json:"manual_approved_until"`
	Reason              string  `json:"reason"`
}

func (in RegisterInput) validate() error {
	if fields := in.problems(); len(fields) > 0 {
		return httpx.Validation(fields)
	}
	return nil
}

func (in RegisterInput) decodePublicKey() ([]byte, bool) {
	if in.PublicKey == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*in.PublicKey)
	if trimmed == "" {
		return nil, true
	}
	if len(trimmed) > maxPublicKeyB64Len {
		return nil, false
	}
	der, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, false
	}
	if _, err := integrity.ParseDevicePublicKey(der); err != nil {
		return nil, false
	}
	return der, true
}

func (in RegisterInput) problems() map[string]string {
	fields := map[string]string{}
	if !hashPattern.MatchString(in.DeviceHash) {
		fields["device_hash"] = "Hash perangkat tidak valid."
	}
	if in.Model != nil && len(*in.Model) > maxModelLen {
		fields["model"] = "Model maksimal 100 karakter."
	}
	if in.OSVersion != nil && len(*in.OSVersion) > maxVersionLen {
		fields["os_version"] = "Versi OS maksimal 50 karakter."
	}
	if in.FCMToken != nil && len(*in.FCMToken) > maxFCMTokenLen {
		fields["fcm_token"] = "Token notifikasi terlalu panjang."
	}
	if _, ok := in.decodePublicKey(); !ok {
		fields["public_key"] = "Kunci publik perangkat harus ECDSA P-256 PKIX DER yang valid."
	}
	return fields
}

func validateTierInput(in TierInput, now time.Time) (store.DeviceIntegrityTier, pgtype.Timestamptz, string, error) {
	fields := map[string]string{}
	tier := store.DeviceIntegrityTier(strings.TrimSpace(in.IntegrityTier))
	switch tier {
	case store.DeviceIntegrityTierPlay, store.DeviceIntegrityTierAttestation, store.DeviceIntegrityTierManual, store.DeviceIntegrityTierKiosk:
	default:
		fields["integrity_tier"] = "Jalur verifikasi tidak valid."
	}
	reason := strings.TrimSpace(in.Reason)
	if len(reason) < minTierReasonLen || len(reason) > maxTierReasonLen {
		fields["reason"] = "Alasan wajib 3 sampai 250 karakter."
	}
	var manualUntil pgtype.Timestamptz
	if tier == store.DeviceIntegrityTierManual {
		if in.ManualApprovedUntil == nil || strings.TrimSpace(*in.ManualApprovedUntil) == "" {
			fields["manual_approved_until"] = "Batas waktu wajib diisi untuk jalur manual."
		} else {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.ManualApprovedUntil))
			if err != nil {
				fields["manual_approved_until"] = "Format waktu tidak valid."
			} else if !parsed.After(now) || parsed.After(now.AddDate(0, 0, maxManualApprovalDay)) {
				fields["manual_approved_until"] = "Masa berlaku jalur manual harus di masa depan dan maksimal 90 hari."
			} else {
				manualUntil = pgconv.Time(parsed.UTC())
			}
		}
	}
	if len(fields) > 0 {
		return "", pgtype.Timestamptz{}, "", httpx.Validation(fields)
	}
	return tier, manualUntil, reason, nil
}

func errRevoked() error {
	return httpx.NewError(http.StatusForbidden, "DEVICE_REVOKED", "Perangkat ini sudah dicabut. Hubungi SDM.")
}

// Service memuat aturan perangkat.
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: store.New(pool)} }

func timeString(t pgtype.Timestamptz) string { return t.Time.UTC().Format(time.RFC3339) }

func present(d store.Device) Device {
	tier := string(d.IntegrityTier)
	if tier == "" {
		tier = string(store.DeviceIntegrityTierPlay)
	}
	return Device{
		ID: d.ID.String(), Model: pgconv.TextPtr(d.Model), OSVersion: pgconv.TextPtr(d.OsVersion),
		Status: string(d.Status), IntegrityTier: tier, ManualApprovedUntil: pgconv.TimeString(d.ManualApprovedUntil),
		KeyBound: len(d.PublicKey) > 0, CreatedAt: timeString(d.CreatedAt), LastSeenAt: pgconv.TimeString(d.LastSeenAt),
	}
}

func presentAdmin(r store.GetAdminDeviceRow) AdminDevice {
	tier := string(r.IntegrityTier)
	if tier == "" {
		tier = string(store.DeviceIntegrityTierPlay)
	}
	return AdminDevice{
		Device: Device{
			ID: r.ID.String(), Model: pgconv.TextPtr(r.Model), OSVersion: pgconv.TextPtr(r.OsVersion),
			Status: string(r.Status), IntegrityTier: tier, ManualApprovedUntil: pgconv.TimeString(r.ManualApprovedUntil),
			KeyBound: r.KeyBound, CreatedAt: timeString(r.CreatedAt), LastSeenAt: pgconv.TimeString(r.LastSeenAt),
		},
		EmployeeID: r.EmployeeID.String(), EmployeeName: r.EmployeeName, Nip: r.Nip,
		SubdivisionName: r.SubdivisionName, DivisionName: r.DivisionName,
		ApprovedAt: pgconv.TimeString(r.ApprovedAt), ApprovedByName: pgconv.TextPtr(r.ApprovedByName),
		RevokedAt: pgconv.TimeString(r.RevokedAt), RevokedReason: pgconv.TextPtr(r.RevokedReason),
		OwnerIsSuperadmin: r.OwnerIsSuperadmin,
	}
}

// Register mencatat perangkat milik pengguna. Hanya akun yang belum pernah punya perangkat
// yang langsung approved. Akun yang perangkatnya pernah dicabut tetap harus disetujui, agar
// pencuri kata sandi tidak mendapat perangkat approved setelah HP dilaporkan hilang (D32).
func (s *Service) Register(ctx context.Context, p auth.Principal, ip *netip.Addr, in RegisterInput) (Device, bool, error) {
	if err := in.validate(); err != nil {
		return Device{}, false, err
	}
	var out store.Device
	created := false
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		out, created, err = bind(ctx, q, p.EmployeeID, p.UserID, ip, in)
		return err
	})
	if err != nil {
		return Device{}, false, err
	}
	return present(out), created, nil
}

// BindForLogin mengikat perangkat saat login mobile, di dalam transaksi pembuatan sesi (F5).
func BindForLogin(ctx context.Context, q *store.Queries, employeeID, userID uuid.UUID, ip *netip.Addr, d auth.MobileDevice) (auth.BoundDevice, error) {
	in := RegisterInput(d)
	if problems := in.problems(); len(problems) > 0 {
		fields := map[string]string{}
		for name, msg := range problems {
			fields["device."+name] = msg
		}
		return auth.BoundDevice{}, httpx.Validation(fields)
	}
	device, _, err := bind(ctx, q, employeeID, userID, ip, in)
	if err != nil {
		return auth.BoundDevice{}, err
	}
	return auth.BoundDevice{ID: device.ID, Status: string(device.Status)}, nil
}

// bind mendaftarkan perangkat baru atau mengenali yang lama. Perangkat langsung approved hanya bila
// akun belum pernah punya perangkat (D32).
func bind(ctx context.Context, q *store.Queries, employeeID, userID uuid.UUID, ip *netip.Addr, in RegisterInput) (store.Device, bool, error) {
	var out store.Device
	created := false
	err := func() error {
		if _, err := q.LockEmployeeForDevice(ctx, employeeID); err != nil {
			if pgconv.IsNoRows(err) {
				return httpx.ErrNotFound()
			}
			return err
		}
		pubKey, _ := in.decodePublicKey()
		existing, err := q.GetDeviceByHash(ctx, store.GetDeviceByHashParams{EmployeeID: employeeID, DeviceHash: in.DeviceHash})
		if err == nil {
			if existing.Status == store.DeviceStatusRevoked {
				return errRevoked()
			}
			// Kunci instalasi yang sudah terikat tidak boleh diganti pada device_hash yang sama (§5.3).
			if len(existing.PublicKey) > 0 && len(pubKey) > 0 && !bytes.Equal(existing.PublicKey, pubKey) {
				return errRevoked()
			}
			out, err = q.TouchDevice(ctx, store.TouchDeviceParams{
				ID: existing.ID, Model: pgconv.Text(in.Model), OsVersion: pgconv.Text(in.OSVersion),
				FcmToken: pgconv.Text(in.FCMToken), PublicKey: pubKey,
			})
			return err
		}
		if !pgconv.IsNoRows(err) {
			return err
		}
		count, err := q.CountEmployeeDevices(ctx, employeeID)
		if err != nil {
			return err
		}
		status := store.DeviceStatusPending
		approvedAt := pgtype.Timestamptz{}
		now := time.Now()
		if count == 0 {
			status = store.DeviceStatusApproved
			approvedAt = pgconv.Time(now)
		}
		keyBoundAt := pgtype.Timestamptz{}
		if len(pubKey) > 0 {
			keyBoundAt = pgconv.Time(now)
		}
		out, err = q.InsertDevice(ctx, store.InsertDeviceParams{
			EmployeeID: employeeID, DeviceHash: in.DeviceHash, Model: pgconv.Text(in.Model),
			OsVersion: pgconv.Text(in.OSVersion), FcmToken: pgconv.Text(in.FCMToken),
			PublicKey: pubKey, KeyBoundAt: keyBoundAt, Status: status, ApprovedAt: approvedAt,
		})
		if err != nil {
			return err
		}
		created = true
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(userID), Action: audit.DeviceRegistered, Entity: "device", EntityID: out.ID,
			Detail: map[string]any{"status": string(status), "key_bound": len(pubKey) > 0}, IP: ip,
		})
	}()
	return out, created, err
}

// Mine mengembalikan semua perangkat milik pengguna.
func (s *Service) Mine(ctx context.Context, p auth.Principal) ([]Device, error) {
	rows, err := s.q.ListMyDevices(ctx, p.EmployeeID)
	if err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(rows))
	for _, r := range rows {
		out = append(out, present(r))
	}
	return out, nil
}

// List mengembalikan perangkat untuk ditinjau, boleh disaring per status.
func (s *Service) List(ctx context.Context, status string, page httpx.Page) ([]AdminDevice, int64, error) {
	filter := store.NullDeviceStatus{}
	if status != "" {
		switch store.DeviceStatus(status) {
		case store.DeviceStatusPending, store.DeviceStatusApproved, store.DeviceStatusRevoked:
			filter = store.NullDeviceStatus{DeviceStatus: store.DeviceStatus(status), Valid: true}
		default:
			return nil, 0, httpx.Validation(map[string]string{"status": "Status tidak valid."})
		}
	}
	total, err := s.q.CountDevices(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListDevices(ctx, store.ListDevicesParams{Status: filter, PageLimit: page.Limit(), PageOffset: page.Offset()})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminDevice, 0, len(rows))
	for _, r := range rows {
		out = append(out, presentAdmin(store.GetAdminDeviceRow(r)))
	}
	return out, total, nil
}

func (s *Service) adminView(ctx context.Context, id uuid.UUID) (AdminDevice, error) {
	row, err := s.q.GetAdminDevice(ctx, id)
	if err != nil {
		return AdminDevice{}, err
	}
	return presentAdmin(row), nil
}

// lockDevice mengunci perangkat lalu pemiliknya, dengan urutan yang sama di setiap aksi.
func lockDevice(ctx context.Context, q *store.Queries, id uuid.UUID) (store.Device, error) {
	d, err := q.GetDeviceForUpdate(ctx, id)
	if pgconv.IsNoRows(err) {
		return d, httpx.ErrNotFound()
	}
	if err != nil {
		return d, err
	}
	if _, err := q.LockEmployeeForDevice(ctx, d.EmployeeID); err != nil && !pgconv.IsNoRows(err) {
		return d, err
	}
	return d, nil
}

// Approve menyetujui perangkat dan mencabut perangkat lama beserta sesinya (F16).
// Peninjau tidak boleh menyetujui perangkatnya sendiri, supaya akun peninjau yang dicuri
// tidak bisa langsung memasang HP baru.
func (s *Service) Approve(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID) (AdminDevice, error) {
	actor := audit.Actor(p.UserID)
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		d, err := lockDevice(ctx, q, id)
		if err != nil {
			return err
		}
		if d.EmployeeID == p.EmployeeID {
			return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Perangkat sendiri harus disetujui peninjau lain.")
		}
		switch d.Status {
		case store.DeviceStatusApproved:
			return nil
		case store.DeviceStatusRevoked:
			return errRevoked()
		}
		replaced, err := q.ReplaceApprovedDevices(ctx, store.ReplaceApprovedDevicesParams{Actor: actor, EmployeeID: d.EmployeeID, KeepID: id})
		if err != nil {
			return err
		}
		for _, old := range replaced {
			if err := q.RevokeDeviceSessions(ctx, store.RevokeDeviceSessionsParams{Reason: pgtype.Text{String: reasonReplaced, Valid: true}, DeviceID: uuid.NullUUID{UUID: old, Valid: true}}); err != nil {
				return err
			}
			if err := audit.Write(ctx, q, audit.Entry{
				UserID: actor, Action: audit.DeviceRevoked, Entity: "device", EntityID: old,
				Detail: map[string]any{"reason": reasonReplaced}, IP: ip,
			}); err != nil {
				return err
			}
		}
		if err := q.ApproveDevice(ctx, store.ApproveDeviceParams{Actor: actor, ID: id}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{UserID: actor, Action: audit.DeviceApproved, Entity: "device", EntityID: id, IP: ip})
	})
	if err != nil {
		return AdminDevice{}, err
	}
	return s.adminView(ctx, id)
}

// Revoke mencabut perangkat beserta sesinya. Mencabut perangkat sendiri boleh, misalnya HP hilang.
// SDM tidak bisa mencabut perangkat super admin (D34).
func (s *Service) Revoke(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID) (AdminDevice, error) {
	actor := audit.Actor(p.UserID)
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		d, err := lockDevice(ctx, q, id)
		if err != nil {
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, d.EmployeeID); err != nil {
			return err
		}
		if d.Status == store.DeviceStatusRevoked {
			return nil
		}
		reason := pgtype.Text{String: reasonAdminRevoked, Valid: true}
		if err := q.RevokeDevice(ctx, store.RevokeDeviceParams{Actor: actor, Reason: reason, ID: id}); err != nil {
			return err
		}
		if err := q.RevokeDeviceSessions(ctx, store.RevokeDeviceSessionsParams{Reason: reason, DeviceID: uuid.NullUUID{UUID: id, Valid: true}}); err != nil {
			return err
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: actor, Action: audit.DeviceRevoked, Entity: "device", EntityID: id,
			Detail: map[string]any{"reason": reasonAdminRevoked}, IP: ip,
		})
	})
	if err != nil {
		return AdminDevice{}, err
	}
	return s.adminView(ctx, id)
}

// Handler melayani /v1/devices dan /v1/admin/devices.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) error {
	var in RegisterInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	in.DeviceHash = strings.ToLower(strings.TrimSpace(in.DeviceHash))
	p, _ := auth.PrincipalFrom(r.Context())
	out, created, err := h.svc.Register(r.Context(), p, audit.ClientIP(r), in)
	if err != nil {
		return err
	}
	if created {
		return httpx.Data(w, http.StatusCreated, out)
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) error {
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := h.svc.Mine(r.Context(), p)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	page, err := httpx.ReadPage(r)
	if err != nil {
		return err
	}
	rows, total, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), page)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, rows, page.Meta(total))
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Approve(r.Context(), p, audit.ClientIP(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.Revoke(r.Context(), p, audit.ClientIP(r), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// SetTier mengubah jalur verifikasi integritas perangkat dan mencatatnya di audit (MASTER.md §5.4.1).
// Aturan anti-turun-jalur: peninjau tidak boleh melonggarkan jalur perangkatnya sendiri.
func (s *Service) SetTier(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in TierInput) (AdminDevice, error) {
	now := time.Now()
	tier, manualUntil, reason, err := validateTierInput(in, now)
	if err != nil {
		return AdminDevice{}, err
	}
	actor := audit.Actor(p.UserID)
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		d, err := lockDevice(ctx, q, id)
		if err != nil {
			return err
		}
		if err := auth.AssertEmployeeNotProtected(ctx, q, p, d.EmployeeID); err != nil {
			return err
		}
		if d.EmployeeID == p.EmployeeID && tier != store.DeviceIntegrityTierPlay {
			return httpx.NewError(http.StatusConflict, "CONFLICT_SELF_ACTION", "Jalur verifikasi perangkat sendiri harus diubah peninjau lain.")
		}
		if d.Status == store.DeviceStatusRevoked {
			return errRevoked()
		}
		if err := q.SetDeviceIntegrityTier(ctx, store.SetDeviceIntegrityTierParams{
			IntegrityTier: tier, ManualApprovedUntil: manualUntil, Actor: actor, ID: id,
		}); err != nil {
			return err
		}
		detail := map[string]any{
			"from_tier": string(d.IntegrityTier),
			"to_tier":   string(tier),
			"reason":    reason,
		}
		if manualUntil.Valid {
			detail["manual_approved_until"] = timeString(manualUntil)
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: actor, Action: audit.DeviceIntegrityTierChanged, Entity: "device", EntityID: id,
			Detail: detail, IP: ip,
		})
	})
	if err != nil {
		return AdminDevice{}, err
	}
	return s.adminView(ctx, id)
}

func (h *Handler) SetTier(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in TierInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, _ := auth.PrincipalFrom(r.Context())
	out, err := h.svc.SetTier(r.Context(), p, audit.ClientIP(r), id, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
