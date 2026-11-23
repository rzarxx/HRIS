package mail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const settingKey = "smtp"

// stored adalah isi app_settings "smtp". Kata sandi disimpan terenkripsi fieldcrypt.
type stored struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordEnc string `json:"password_enc"`
	FromName    string `json:"from_name"`
	FromEmail   string `json:"from_email"`
}

// Service menyimpan pengaturan SMTP dan mengirim email dengannya.
type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	cipher *fieldcrypt.Cipher
	sender interface {
		Send(ctx context.Context, cfg Config, msg Message) error
	}
}

func NewService(pool *pgxpool.Pool, cipher *fieldcrypt.Cipher) *Service {
	return &Service{pool: pool, q: store.New(pool), cipher: cipher, sender: NewSender()}
}

// TransportFunc mengirim satu email dengan pengaturan tertentu; dipakai tes untuk mengganti SMTP sungguhan.
type TransportFunc func(ctx context.Context, cfg Config, msg Message) error

func (f TransportFunc) Send(ctx context.Context, cfg Config, msg Message) error {
	return f(ctx, cfg, msg)
}

// WithTransport mengganti pengirim SMTP (untuk tes).
func (s *Service) WithTransport(t TransportFunc) *Service {
	s.sender = t
	return s
}

// View adalah pengaturan SMTP untuk panel. Kata sandi tidak pernah dikirim balik.
type View struct {
	Configured  bool    `json:"configured"`
	Host        string  `json:"host"`
	Port        int     `json:"port"`
	Username    string  `json:"username"`
	PasswordSet bool    `json:"password_set"`
	FromName    string  `json:"from_name"`
	FromEmail   string  `json:"from_email"`
	UpdatedAt   *string `json:"updated_at"`
}

// Input adalah isian PUT. Password kosong berarti tetap memakai kata sandi lama.
type Input struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
}

func (s *Service) load(ctx context.Context) (stored, *string, bool, error) {
	row, err := s.q.GetAppSetting(ctx, settingKey)
	if pgconv.IsNoRows(err) {
		return stored{}, nil, false, nil
	}
	if err != nil {
		return stored{}, nil, false, err
	}
	var st stored
	if err := json.Unmarshal([]byte(row.Value), &st); err != nil {
		return stored{}, nil, false, err
	}
	return st, pgconv.TimeString(row.UpdatedAt), true, nil
}

func (s *Service) decrypt(st stored) (Config, error) {
	cfg := Config{Host: st.Host, Port: st.Port, Username: st.Username, FromName: st.FromName, FromEmail: st.FromEmail}
	if st.PasswordEnc != "" {
		raw, err := base64.StdEncoding.DecodeString(st.PasswordEnc)
		if err != nil {
			return cfg, err
		}
		plain, err := s.cipher.Decrypt(raw)
		if err != nil {
			return cfg, err
		}
		cfg.Password = string(plain)
	}
	return cfg, nil
}

// Settings mengembalikan pengaturan SMTP tanpa kata sandi.
func (s *Service) Settings(ctx context.Context) (View, error) {
	st, updated, ok, err := s.load(ctx)
	if err != nil || !ok {
		return View{}, err
	}
	return View{
		Configured: true, Host: st.Host, Port: st.Port, Username: st.Username, PasswordSet: st.PasswordEnc != "",
		FromName: st.FromName, FromEmail: st.FromEmail, UpdatedAt: updated,
	}, nil
}

func (in *Input) normalize() map[string]string {
	in.Host = strings.ToLower(strings.TrimSpace(in.Host))
	in.Username = strings.TrimSpace(in.Username)
	in.FromName = strings.TrimSpace(in.FromName)
	in.FromEmail = strings.TrimSpace(in.FromEmail)
	fields := map[string]string{}
	if !hostPattern.MatchString(in.Host) || len(in.Host) > 253 || net.ParseIP(in.Host) != nil {
		fields["host"] = "Tulis nama host SMTP, contoh smtp.gmail.com."
	}
	if in.Port != 465 && in.Port != 587 {
		fields["port"] = "Port SMTP 465 (SSL/TLS) atau 587 (STARTTLS)."
	}
	if len(in.Username) > 254 || strings.ContainsAny(in.Username, "\r\n") {
		fields["username"] = "Username maksimal 254 karakter."
	}
	if len(in.Password) > 256 || strings.ContainsAny(in.Password, "\r\n") {
		fields["password"] = "Kata sandi SMTP maksimal 256 karakter."
	}
	if utf8.RuneCountInString(in.FromName) > 80 || strings.ContainsAny(in.FromName, "\r\n") {
		fields["from_name"] = "Nama pengirim maksimal 80 karakter."
	}
	if addr, err := mail.ParseAddress(in.FromEmail); err != nil || addr.Address != in.FromEmail || !ValidSender(in.FromEmail) {
		fields["from_email"] = "Tulis email pengirim yang sesuai, contoh hris@albahjah.or.id."
	}
	return fields
}

func errTestFailed(stage string) error {
	msg := "Email uji belum terkirim. Periksa host, port, username, dan kata sandi SMTP."
	switch stage {
	case StageConnect:
		msg = "Server SMTP belum dapat dihubungi. Periksa host dan port, atau pastikan alamatnya bukan alamat internal."
	case StageTLS:
		msg = "Koneksi aman (TLS) ke server SMTP belum berhasil. Periksa host dan port."
	case StageAuth:
		msg = "Username atau kata sandi SMTP belum sesuai."
	case StageSend:
		msg = "Server SMTP menolak pengiriman. Periksa email pengirim dan izin akun SMTP."
	}
	e := httpx.NewError(http.StatusUnprocessableEntity, "SMTP_TEST_FAILED", msg)
	e.Details = map[string]any{"stage": stage}
	return e
}

// Save mengirim email uji ke testTo memakai pengaturan baru, lalu menyimpannya hanya bila uji berhasil.
// Bila gagal, pengaturan lama tetap dipakai.
func (s *Service) Save(ctx context.Context, entry audit.Entry, testTo string, in Input) (View, error) {
	fields := in.normalize()
	if strings.TrimSpace(testTo) == "" {
		fields["test_to"] = "Isi email akun Anda lebih dulu (Pengaturan → Akun saya) untuk menerima email uji."
	}
	if len(fields) > 0 {
		return View{}, httpx.Validation(fields)
	}
	old, _, had, err := s.load(ctx)
	if err != nil {
		return View{}, err
	}
	cfg := Config{Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, FromName: in.FromName, FromEmail: in.FromEmail}
	passwordEnc := ""
	switch {
	case in.Password != "":
		enc, err := s.cipher.Encrypt([]byte(in.Password))
		if err != nil {
			return View{}, err
		}
		passwordEnc = base64.StdEncoding.EncodeToString(enc)
	case had && old.PasswordEnc != "":
		prev, err := s.decrypt(old)
		if err != nil {
			return View{}, err
		}
		cfg.Password = prev.Password
		passwordEnc = old.PasswordEnc
	}
	if cfg.Username != "" && cfg.Password == "" {
		return View{}, httpx.Validation(map[string]string{"password": "Kata sandi SMTP wajib diisi."})
	}
	err = s.sender.Send(ctx, cfg, Message{
		To:      testTo,
		Subject: "Uji email HRIS Al Bahjah",
		Body:    "Pengaturan email HRIS Al Bahjah berhasil. Email akun seperti reset kata sandi kini dikirim dengan pengaturan ini.\n\nEmail ini dikirim otomatis; mohon tidak dibalas.",
	})
	if err != nil {
		var se *SendError
		if errors.As(err, &se) {
			if se.Stage == StageAddress {
				return View{}, httpx.Validation(map[string]string{"test_to": "Email akun Anda belum sesuai untuk menerima email uji."})
			}
			return View{}, errTestFailed(se.Stage)
		}
		return View{}, errTestFailed(StageSend)
	}
	value, err := json.Marshal(stored{
		Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, PasswordEnc: passwordEnc, FromName: cfg.FromName, FromEmail: cfg.FromEmail,
	})
	if err != nil {
		return View{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: settingKey, Value: string(value), UpdatedBy: entry.UserID}); err != nil {
			return err
		}
		e := entry
		e.Action, e.Entity = audit.SettingsChanged, "app_setting"
		// Tanpa nilai rahasia: hanya nama field yang berubah dan apakah kata sandi diganti.
		e.Detail = map[string]any{"key": settingKey, "host": cfg.Host, "port": cfg.Port, "password_changed": in.Password != ""}
		return audit.Write(ctx, q, e)
	})
	if err != nil {
		return View{}, err
	}
	return s.Settings(ctx)
}

// Disable menghapus pengaturan SMTP sehingga email akun tidak dikirim.
func (s *Service) Disable(ctx context.Context, entry audit.Entry) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		n, err := q.DeleteAppSetting(ctx, settingKey)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.ErrNotFound()
		}
		e := entry
		e.Action, e.Entity = audit.SettingsChanged, "app_setting"
		e.Detail = map[string]any{"key": settingKey, "change": "disabled"}
		return audit.Write(ctx, q, e)
	})
}

// ErrNotConfigured berarti SMTP belum diatur.
var ErrNotConfigured = errors.New("SMTP belum diatur")

// Enabled menjawab apakah SMTP sudah diatur.
func (s *Service) Enabled(ctx context.Context) bool {
	_, _, ok, err := s.load(ctx)
	return err == nil && ok
}

// Send mengirim satu email dengan pengaturan tersimpan.
func (s *Service) Send(ctx context.Context, to, subject, body string) error {
	st, _, ok, err := s.load(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotConfigured
	}
	cfg, err := s.decrypt(st)
	if err != nil {
		return err
	}
	return s.sender.Send(ctx, cfg, Message{To: to, Subject: subject, Body: body})
}
