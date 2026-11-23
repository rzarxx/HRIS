// Package mail mengirim email akun (reset kata sandi dan pemberitahuan keamanan) lewat SMTP yang
// diatur super admin di panel (D30, MASTER.md §5.2.1).
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config adalah pengaturan SMTP yang sudah didekripsi.
type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
}

// Message adalah satu email teks biasa.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Tahap kegagalan, ditampilkan ke super admin agar tahu apa yang perlu diperiksa. Isi galat dari server
// SMTP tidak diteruskan karena bisa memuat data internal.
const (
	StageAddress = "address"
	StageConnect = "connect"
	StageTLS     = "tls"
	StageAuth    = "auth"
	StageSend    = "send"
)

// SendError menandai tahap pengiriman yang gagal.
type SendError struct {
	Stage string
	Err   error
}

func (e *SendError) Error() string { return "smtp " + e.Stage + ": " + e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

const sendTimeout = 20 * time.Second

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$`)

// ErrHostNotAllowed berarti host SMTP mengarah ke alamat internal server.
var ErrHostNotAllowed = errors.New("alamat SMTP tidak diizinkan")

// allowedIP menolak alamat yang bisa dipakai menjangkau layanan internal server (loopback,
// link-local, jaringan privat, dan sejenisnya).
func allowedIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast())
}

// Sender mengirim email lewat SMTP. Field selain bawaan hanya diganti di tes.
type Sender struct {
	lookup  func(ctx context.Context, host string) ([]net.IP, error)
	dial    func(ctx context.Context, network, addr string) (net.Conn, error)
	roots   *x509.CertPool
	allowIP func(net.IP) bool
	now     func() time.Time
}

// NewSender membuat pengirim dengan resolver dan dialer sistem.
func NewSender() *Sender {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &Sender{
		lookup: func(ctx context.Context, host string) ([]net.IP, error) {
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			ips := make([]net.IP, 0, len(addrs))
			for _, a := range addrs {
				ips = append(ips, a.IP)
			}
			return ips, nil
		},
		dial:    d.DialContext,
		allowIP: allowedIP,
		now:     time.Now,
	}
}

// resolve mengembalikan alamat pertama yang diizinkan. Koneksi dibuka ke alamat ini, bukan nama host,
// agar nama host tidak bisa diarahkan ulang ke alamat internal di antara pemeriksaan dan koneksi.
func (s *Sender) resolve(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return nil, ErrHostNotAllowed
	}
	ips, err := s.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("host SMTP tidak ditemukan")
	}
	for _, ip := range ips {
		if !s.allowIP(ip) {
			return nil, ErrHostNotAllowed
		}
	}
	return ips[0], nil
}

// Send mengirim satu email. Port 465 memakai TLS langsung; port 587 wajib STARTTLS. Sertifikat server
// selalu diverifikasi terhadap nama host.
func (s *Sender) Send(ctx context.Context, cfg Config, msg Message) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := checkHeader(msg.To); err != nil {
		return &SendError{Stage: StageAddress, Err: err}
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return &SendError{Stage: StageAddress, Err: err}
	}
	data, err := s.build(cfg, msg)
	if err != nil {
		return &SendError{Stage: StageAddress, Err: err}
	}
	ip, err := s.resolve(ctx, cfg.Host)
	if err != nil {
		return &SendError{Stage: StageConnect, Err: err}
	}
	conn, err := s.dial(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(cfg.Port)))
	if err != nil {
		return &SendError{Stage: StageConnect, Err: err}
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12, RootCAs: s.roots}
	if cfg.Port == 465 {
		tc := tls.Client(conn, tlsConfig)
		if err := tc.HandshakeContext(ctx); err != nil {
			return &SendError{Stage: StageTLS, Err: err}
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return &SendError{Stage: StageConnect, Err: err}
	}
	defer c.Close()
	if err := c.Hello("localhost"); err != nil {
		return &SendError{Stage: StageConnect, Err: err}
	}
	if cfg.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return &SendError{Stage: StageTLS, Err: errors.New("server tidak mendukung STARTTLS")}
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return &SendError{Stage: StageTLS, Err: err}
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return &SendError{Stage: StageAuth, Err: err}
		}
	}
	if err := c.Mail(cfg.FromEmail); err != nil {
		return &SendError{Stage: StageSend, Err: err}
	}
	if err := c.Rcpt(msg.To); err != nil {
		return &SendError{Stage: StageSend, Err: err}
	}
	w, err := c.Data()
	if err != nil {
		return &SendError{Stage: StageSend, Err: err}
	}
	if _, err := w.Write(data); err != nil {
		return &SendError{Stage: StageSend, Err: err}
	}
	if err := w.Close(); err != nil {
		return &SendError{Stage: StageSend, Err: err}
	}
	return c.Quit()
}

// checkHeader menolak nilai yang bisa menyisipkan header baru.
func checkHeader(v string) error {
	if strings.ContainsAny(v, "\r\n") {
		return errors.New("nilai header memuat baris baru")
	}
	return nil
}

// build menyusun email teks biasa UTF-8 dengan badan base64 sehingga isinya tidak bisa membentuk header.
func (s *Sender) build(cfg Config, msg Message) ([]byte, error) {
	for _, v := range []string{cfg.FromName, cfg.FromEmail, msg.To, msg.Subject} {
		if err := checkHeader(v); err != nil {
			return nil, err
		}
	}
	from := (&mail.Address{Name: cfg.FromName, Address: cfg.FromEmail}).String()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	domain := cfg.FromEmail[strings.LastIndex(cfg.FromEmail, "@")+1:]
	var b bytes.Buffer
	headers := [][2]string{
		{"From", from},
		{"To", msg.To},
		{"Subject", mime.QEncoding.Encode("utf-8", msg.Subject)},
		{"Date", s.now().UTC().Format(time.RFC1123Z)},
		{"Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(id), domain)},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=utf-8"},
		{"Content-Transfer-Encoding", "base64"},
		{"Auto-Submitted", "auto-generated"},
	}
	for _, h := range headers {
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	b.WriteString("\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(msg.Body, "\n", "\r\n")))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.Bytes(), nil
}
