package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- RFC 6238 memakai HMAC-SHA1, aplikasi authenticator hanya mendukung ini
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod = 30
	totpDigits = 6
	totpWindow = 1 // toleransi selisih jam ±30 detik
	totpIssuer = "HRIS Pejuang"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret membuat secret 160-bit sesuai rekomendasi RFC 4226.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// TOTPURL membentuk otpauth:// untuk kode QR.
func TOTPURL(secret, account string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", totpIssuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprint(totpDigits))
	v.Set("period", fmt.Sprint(totpPeriod))
	label := url.PathEscape(totpIssuer + ":" + account)
	return "otpauth://totp/" + label + "?" + v.Encode()
}

func hotp(secret []byte, step uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], step)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, code%1_000_000)
}

// VerifyTOTP mengembalikan langkah waktu yang cocok agar pemanggil bisa menolak kode yang
// sudah pernah dipakai. Langkah di bawah atau sama dengan lastStep tidak diterima.
func VerifyTOTP(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return 0, false
	}
	key, err := b32.DecodeString(secret)
	if err != nil {
		return 0, false
	}
	current := now.Unix() / totpPeriod
	for delta := int64(-totpWindow); delta <= totpWindow; delta++ {
		step := current + delta
		if step <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hotp(key, uint64(step))), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// NewRecoveryCodes membuat kode pemulihan acak beserta hash-nya. Kode asli hanya tampil sekali.
func NewRecoveryCodes(n int) (codes, hashes []string, err error) {
	for i := 0; i < n; i++ {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		raw := strings.ToLower(b32.EncodeToString(b))
		code := raw[:8] + "-" + raw[8:]
		codes = append(codes, code)
		hashes = append(hashes, HashToken(code))
	}
	return codes, hashes, nil
}

// ConsumeRecoveryCode mengembalikan daftar hash tanpa kode yang dipakai.
func ConsumeRecoveryCode(hashes []string, code string) ([]string, bool) {
	want := HashToken(strings.ToLower(strings.TrimSpace(code)))
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			rest := make([]string, 0, len(hashes)-1)
			rest = append(rest, hashes[:i]...)
			return append(rest, hashes[i+1:]...), true
		}
	}
	return hashes, false
}
