package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const testHost = "smtp.example.test"

func testCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: testHost}, DNSNames: []string{testHost},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

// fakeSMTP adalah server SMTP minimal untuk tes: TLS langsung (implicit) atau STARTTLS.
type fakeSMTP struct {
	addr     string
	mu       sync.Mutex
	data     string
	auth     string
	startTLS bool
}

func startFakeSMTP(t *testing.T, cert tls.Certificate, implicit, offerStartTLS bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeSMTP{addr: ln.Addr().String()}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn, tlsConfig, implicit, offerStartTLS)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(conn net.Conn, cfg *tls.Config, implicit, offerStartTLS bool) {
	defer conn.Close()
	secure := false
	if implicit {
		conn = tls.Server(conn, cfg)
		secure = true
	}
	r := bufio.NewReader(conn)
	write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
	write("220 " + testHost + " ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		upper := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			if !secure && offerStartTLS {
				write("250-" + testHost)
				write("250-STARTTLS")
				write("250 AUTH PLAIN")
			} else {
				write("250-" + testHost)
				write("250 AUTH PLAIN")
			}
		case upper == "STARTTLS":
			write("220 siap")
			conn = tls.Server(conn, cfg)
			r = bufio.NewReader(conn)
			secure = true
			f.mu.Lock()
			f.startTLS = true
			f.mu.Unlock()
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			f.mu.Lock()
			f.auth = strings.TrimSpace(cmd[len("AUTH PLAIN"):])
			f.mu.Unlock()
			write("235 ok")
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
			write("250 ok")
		case upper == "DATA":
			write("354 lanjut")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.data = b.String()
			f.mu.Unlock()
			write("250 terkirim")
		case upper == "QUIT":
			write("221 bye")
			return
		default:
			write("502 tidak dikenal")
		}
	}
}

func testSender(roots *x509.CertPool, target string) *Sender {
	s := NewSender()
	s.lookup = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.10")}, nil }
	s.dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	s.roots = roots
	return s
}

func testConfig(port int) Config {
	return Config{Host: testHost, Port: port, Username: "hris@albahjah.or.id", Password: "rahasia", FromName: "HRIS Al Bahjah", FromEmail: "hris@albahjah.or.id"}
}

func TestSendStartTLS(t *testing.T) {
	cert, roots := testCert(t)
	srv := startFakeSMTP(t, cert, false, true)
	err := testSender(roots, srv.addr).Send(context.Background(), testConfig(587), Message{To: "siti@pejuang.id", Subject: "Atur ulang kata sandi", Body: "Halo\nBaris dua"})
	if err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !srv.startTLS {
		t.Error("port 587 harus memakai STARTTLS")
	}
	plain, _ := base64.StdEncoding.DecodeString(srv.auth)
	if string(plain) != "\x00hris@albahjah.or.id\x00rahasia" {
		t.Errorf("auth = %q", plain)
	}
	if !strings.Contains(srv.data, "Subject: Atur ulang kata sandi") || !strings.Contains(srv.data, "To: siti@pejuang.id") {
		t.Errorf("header email: %s", srv.data)
	}
}

func TestSendImplicitTLS(t *testing.T) {
	cert, roots := testCert(t)
	srv := startFakeSMTP(t, cert, true, false)
	if err := testSender(roots, srv.addr).Send(context.Background(), testConfig(465), Message{To: "siti@pejuang.id", Subject: "Uji", Body: "Isi"}); err != nil {
		t.Fatal(err)
	}
}

func stageOf(err error) string {
	var se *SendError
	if errors.As(err, &se) {
		return se.Stage
	}
	return ""
}

func TestSendRefusals(t *testing.T) {
	cert, roots := testCert(t)

	// Server tanpa STARTTLS di port 587: tidak boleh mengirim kata sandi tanpa enkripsi.
	plain := startFakeSMTP(t, cert, false, false)
	if err := testSender(roots, plain.addr).Send(context.Background(), testConfig(587), Message{To: "a@b.id", Subject: "x", Body: "y"}); stageOf(err) != StageTLS {
		t.Errorf("tanpa STARTTLS: %v", err)
	}
	// Sertifikat yang tidak dipercaya ditolak.
	tlsSrv := startFakeSMTP(t, cert, true, false)
	if err := testSender(x509.NewCertPool(), tlsSrv.addr).Send(context.Background(), testConfig(465), Message{To: "a@b.id", Subject: "x", Body: "y"}); stageOf(err) != StageTLS {
		t.Errorf("sertifikat tak dipercaya: %v", err)
	}
	// Host yang mengarah ke alamat internal atau berupa IP ditolak sebelum koneksi.
	s := testSender(roots, tlsSrv.addr)
	for _, ip := range []string{"127.0.0.1", "10.0.0.5", "192.168.1.2", "169.254.169.254", "::1", "0.0.0.0"} {
		s.lookup = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP(ip)}, nil }
		if err := s.Send(context.Background(), testConfig(465), Message{To: "a@b.id", Subject: "x", Body: "y"}); !errors.Is(err, ErrHostNotAllowed) {
			t.Errorf("alamat %s tidak ditolak: %v", ip, err)
		}
	}
	ipCfg := testConfig(465)
	ipCfg.Host = "203.0.113.10"
	if err := s.Send(context.Background(), ipCfg, Message{To: "a@b.id", Subject: "x", Body: "y"}); !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("host IP tidak ditolak: %v", err)
	}
	// Header tidak bisa disisipi baris baru.
	for _, msg := range []Message{
		{To: "a@b.id\r\nBcc: x@y.id", Subject: "x", Body: "y"},
		{To: "a@b.id", Subject: "x\r\nBcc: x@y.id", Body: "y"},
	} {
		if err := s.Send(context.Background(), testConfig(465), msg); stageOf(err) != StageAddress {
			t.Errorf("sisipan header tidak ditolak: %v", err)
		}
	}
}

func TestInputValidation(t *testing.T) {
	ok := Input{Host: "smtp.gmail.com", Port: 587, Username: "u", Password: "p", FromName: "HRIS", FromEmail: "hris@albahjah.or.id"}
	if f := ok.normalize(); len(f) != 0 {
		t.Fatalf("input sah ditolak: %v", f)
	}
	for name, in := range map[string]Input{
		"port 25":    {Host: "smtp.gmail.com", Port: 25, FromEmail: "a@b.id"},
		"host ip":    {Host: "10.0.0.1", Port: 587, FromEmail: "a@b.id"},
		"host spasi": {Host: "smtp gmail.com", Port: 587, FromEmail: "a@b.id"},
		"from salah": {Host: "smtp.gmail.com", Port: 587, FromEmail: "bukan email"},
		"nama CRLF":  {Host: "smtp.gmail.com", Port: 587, FromEmail: "a@b.id", FromName: "a\r\nb"},
	} {
		if f := in.normalize(); len(f) == 0 {
			t.Errorf("%s diterima", name)
		}
	}
}
