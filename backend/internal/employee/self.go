package employee

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/platform/phone"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// PhotoStore menyimpan foto profil terenkripsi. DiskStore presensi (folder SELFIE_DIR) memenuhinya;
// foto profil memakai awalan kunci profile/ agar terpisah dari selfie presensi.
type PhotoStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// WithPhotos memasang penyimpanan foto profil. Tanpa ini, endpoint foto menjawab STORAGE_UNAVAILABLE.
func (s *Service) WithPhotos(store PhotoStore) *Service {
	s.photos = store
	return s
}

func errStorageUnavailable() error {
	return httpx.NewError(http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Penyimpanan foto belum tersedia. Silakan hubungi SDM.")
}

// ContactInput adalah body PATCH /v1/me. Field yang tidak dikirim tetap nilai lamanya.
type ContactInput struct {
	Phone           *string `json:"phone"`
	Email           *string `json:"email"`
	CurrentPassword string  `json:"current_password"`
}

// self membaca data Pejuang milik pemanggil tanpa batas scope.
func (s *Service) self(ctx context.Context, p auth.Principal) (store.GetEmployeeScopedRow, error) {
	if p.EmployeeID == uuid.Nil {
		return store.GetEmployeeScopedRow{}, httpx.ErrForbidden()
	}
	row, err := s.q.GetEmployeeScoped(ctx, store.GetEmployeeScopedParams{ID: p.EmployeeID, ScopeAll: true})
	if pgconv.IsNoRows(err) {
		return row, httpx.ErrNotFound()
	}
	return row, err
}

// UpdateOwnContact mengubah nomor WA dan email milik sendiri. Ganti email wajib kata sandi saat ini
// karena email juga dipakai untuk masuk. Bila SMTP aktif, email baru menunggu konfirmasi lewat tautan
// di email itu (D59); tanpa SMTP, email login berubah langsung dalam transaksi yang sama.
func (s *Service) UpdateOwnContact(ctx context.Context, p auth.Principal, ip audit.Entry, decode func(*ContactInput) error) error {
	current, err := s.self(ctx, p)
	if err != nil {
		return err
	}
	in := ContactInput{Phone: pgconv.TextPtr(current.Phone), Email: pgconv.TextPtr(current.Email)}
	if err := decode(&in); err != nil {
		return err
	}
	fields := map[string]string{}
	email := pgtype.Text{}
	if in.Email != nil {
		if v := strings.TrimSpace(*in.Email); v != "" {
			email = pgtype.Text{String: v, Valid: true}
		}
	}
	if email.Valid && (len(email.String) > 254 || !emailPattern.MatchString(email.String)) {
		fields["email"] = "Mohon periksa kembali format email, contoh nama@contoh.id."
	}
	wa := pgtype.Text{}
	if in.Phone != nil && strings.TrimSpace(*in.Phone) != "" {
		normalized, ok := phone.Normalize(*in.Phone)
		if !ok {
			fields["phone"] = "Mohon periksa kembali nomor WA, contoh 081234567890."
		}
		wa = pgtype.Text{String: normalized, Valid: ok}
	}
	emailChanged := !strings.EqualFold(email.String, current.Email.String) || email.Valid != current.Email.Valid
	if emailChanged && len(fields) == 0 {
		if in.CurrentPassword == "" {
			fields["current_password"] = "Masukkan kata sandi saat ini untuk mengganti email."
		} else {
			user, err := s.q.GetUserByID(ctx, p.UserID)
			if err != nil {
				return err
			}
			ok, err := auth.VerifyPassword(in.CurrentPassword, user.PasswordHash)
			if err != nil {
				return err
			}
			if !ok {
				fields["current_password"] = "Kata sandi saat ini belum sesuai. Silakan periksa kembali."
			}
		}
	}
	if len(fields) > 0 {
		return httpx.Validation(fields)
	}
	// Email baru diuji dulu lewat tautan konfirmasi; email lama tetap dipakai sampai tautan dibuka.
	if emailChanged && email.Valid && s.emails != nil && s.emails.EmailConfirmationEnabled(ctx) {
		if err := s.emails.RequestEmailChange(ctx, p.UserID, p.EmployeeID, current.Email.String, email.String, ip.IP, time.Now()); err != nil {
			return err
		}
		emailChanged, email = false, current.Email
	}
	changed := []string{}
	if emailChanged {
		changed = append(changed, "email")
	}
	if wa != current.Phone {
		changed = append(changed, "phone")
	}
	if len(changed) == 0 {
		return nil
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		affected, err := q.UpdateOwnContact(ctx, store.UpdateOwnContactParams{ID: p.EmployeeID, Email: email, Phone: wa})
		if err != nil {
			return mapUnique(err)
		}
		if affected == 0 {
			return httpx.ErrNotFound()
		}
		if emailChanged {
			if err := q.UpdateUserEmail(ctx, store.UpdateUserEmailParams{EmployeeID: p.EmployeeID, Email: email}); err != nil {
				return mapUnique(err)
			}
			if err := recordEmailChange(ctx, q, ip, p.EmployeeID, "self"); err != nil {
				return err
			}
		}
		return writeSelfAudit(ctx, q, ip, p.EmployeeID, changed)
	})
	if err == nil && emailChanged && s.emails != nil {
		s.emails.NotifyEmailChanged(ctx, p.EmployeeID, current.Email.String, email.String, false)
	}
	return err
}

// writeSelfAudit mencatat perubahan oleh Pejuang sendiri; isinya nama field saja, bukan nilainya.
func writeSelfAudit(ctx context.Context, q *store.Queries, ip audit.Entry, id uuid.UUID, fields []string) error {
	entry := ip
	entry.Action, entry.Entity, entry.EntityID = audit.EmployeeUpdated, "employee", id
	entry.Detail = map[string]any{"fields": fields, "self": true}
	return audit.Write(ctx, q, entry)
}

// Family adalah data anak milik sendiri (GET /v1/me/children). Hanya SDM yang mengubahnya.
type Family struct {
	HasChildren   *bool   `json:"has_children"`
	ChildrenCount *int    `json:"children_count"`
	Children      []Child `json:"children"`
}

func (s *Service) OwnFamily(ctx context.Context, p auth.Principal) (Family, error) {
	if p.EmployeeID == uuid.Nil {
		return Family{}, httpx.ErrForbidden()
	}
	row, err := s.q.GetOwnFamily(ctx, p.EmployeeID)
	if pgconv.IsNoRows(err) {
		return Family{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Family{}, err
	}
	children, err := s.ListChildren(ctx, p.EmployeeID)
	if err != nil {
		return Family{}, err
	}
	f := Family{Children: children}
	if row.HasChildren.Valid {
		v := row.HasChildren.Bool
		f.HasChildren = &v
	}
	if row.ChildrenCount.Valid {
		v := int(row.ChildrenCount.Int16)
		f.ChildrenCount = &v
	}
	return f, nil
}

// SetOwnPhoto menyimpan foto profil baru, lalu menghapus berkas foto lama.
func (s *Service) SetOwnPhoto(ctx context.Context, p auth.Principal, ip audit.Entry, data []byte, now time.Time) error {
	if p.EmployeeID == uuid.Nil {
		return httpx.ErrForbidden()
	}
	if s.photos == nil {
		return errStorageUnavailable()
	}
	jpg, err := processPhoto(data)
	switch {
	case errors.Is(err, errPhotoFormat):
		return httpx.Validation(map[string]string{"photo": "Mohon gunakan foto JPEG atau PNG."})
	case errors.Is(err, errPhotoSize):
		return httpx.Validation(map[string]string{"photo": fmt.Sprintf("Ukuran foto minimal %d x %d piksel.", photoMinSide, photoMinSide)})
	case err != nil:
		return err
	}
	key := fmt.Sprintf("profile/%s/%s.jpg", p.EmployeeID, uuid.New())
	if err := s.photos.Put(ctx, key, jpg); err != nil {
		return err
	}
	previous, err := s.swapPhoto(ctx, p.EmployeeID, ip, pgtype.Text{String: key, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}, "photo_set")
	if err != nil {
		s.deletePhoto(ctx, key)
		return err
	}
	s.deletePhoto(ctx, previous)
	return nil
}

// DeleteOwnPhoto menghapus foto profil milik sendiri.
func (s *Service) DeleteOwnPhoto(ctx context.Context, p auth.Principal, ip audit.Entry) error {
	if p.EmployeeID == uuid.Nil {
		return httpx.ErrForbidden()
	}
	previous, err := s.swapPhoto(ctx, p.EmployeeID, ip, pgtype.Text{}, pgtype.Timestamptz{}, "photo_deleted")
	if err != nil {
		return err
	}
	s.deletePhoto(ctx, previous)
	return nil
}

func (s *Service) swapPhoto(ctx context.Context, id uuid.UUID, ip audit.Entry, key pgtype.Text, at pgtype.Timestamptz, field string) (string, error) {
	var previous string
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		old, err := q.SetEmployeePhoto(ctx, store.SetEmployeePhotoParams{ID: id, PhotoKey: key, PhotoUpdatedAt: at})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		previous = old.String
		return writeSelfAudit(ctx, q, ip, id, []string{field})
	})
	return previous, err
}

// deletePhoto menghapus berkas foto; kegagalan hanya dicatat karena datanya sudah tidak dirujuk.
func (s *Service) deletePhoto(ctx context.Context, key string) {
	if key == "" || s.photos == nil {
		return
	}
	if err := s.photos.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "gagal menghapus berkas foto profil lama", "err", err)
	}
}

// Photo mengembalikan foto profil JPEG. Foto sendiri cukup profile:read; foto Pejuang lain butuh
// employee:read dalam scope pemanggil, dan di luar scope dijawab 404 agar keberadaannya tidak bocor.
func (s *Service) Photo(ctx context.Context, p auth.Principal, id uuid.UUID) ([]byte, error) {
	if s.photos == nil {
		return nil, errStorageUnavailable()
	}
	if id != p.EmployeeID {
		if !permission.Allows(p.Role, permission.EmployeeRead) {
			return nil, httpx.ErrNotFound()
		}
		if _, err := s.getScoped(ctx, p, id); err != nil {
			return nil, err
		}
	}
	key, err := s.q.GetEmployeePhoto(ctx, id)
	if pgconv.IsNoRows(err) || (err == nil && !key.Valid) {
		return nil, httpx.ErrNotFound()
	}
	if err != nil {
		return nil, err
	}
	return s.photos.Get(ctx, key.String)
}
