package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("kata-sandi-benar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash bukan argon2id: %s", hash)
	}
	if ok, err := VerifyPassword("kata-sandi-benar", hash); err != nil || !ok {
		t.Errorf("sandi benar ditolak: ok=%v err=%v", ok, err)
	}
	if ok, err := VerifyPassword("kata-sandi-salah", hash); err != nil || ok {
		t.Errorf("sandi salah diterima: ok=%v err=%v", ok, err)
	}
	if _, err := VerifyPassword("x", "bukan-hash"); err == nil {
		t.Error("format hash salah harus error")
	}
}

func TestPasswordHashIsSalted(t *testing.T) {
	a, _ := HashPassword("sama")
	b, _ := HashPassword("sama")
	if a == b {
		t.Error("hash sandi yang sama harus berbeda karena salt")
	}
}

func newSigner(t *testing.T) *TokenSigner {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return NewTokenSigner(key, "test")
}

func TestTokenClaims(t *testing.T) {
	s := newSigner(t)
	userID, employeeID, sessionID := uuid.New(), uuid.New(), uuid.New()
	raw, err := s.Sign(userID, employeeID, sessionID, "sdm", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Verify(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != userID.String() || c.EID != employeeID.String() || c.SID != sessionID.String() || c.Role != "sdm" {
		t.Errorf("klaim salah: %+v", c)
	}
	if c.Issuer != Issuer {
		t.Errorf("iss = %s", c.Issuer)
	}
}

// Token alg none atau HS256 tidak boleh diterima walau klaimnya benar.
func TestVerifyRejectsOtherAlgorithms(t *testing.T) {
	s := newSigner(t)
	claims := Claims{Role: "sdm", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: Issuer, Audience: jwt.ClaimStrings{AudienceWeb}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify(none); err == nil {
		t.Error("alg none diterima")
	}
	hs, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.public))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify(hs); err == nil {
		t.Error("alg HS256 dengan kunci publik diterima")
	}
}

func TestVerifyRejectsWrongAudience(t *testing.T) {
	s := newSigner(t)
	claims := Claims{Role: "sdm", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: Issuer, Audience: jwt.ClaimStrings{"hris-mobile"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.private)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify(raw); err == nil {
		t.Error("aud salah diterima")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "key.pem")
	if _, err := LoadOrCreateKey(path, false); err == nil {
		t.Error("tanpa izin membuat, kunci yang tidak ada harus error")
	}
	first, err := LoadOrCreateKey(path, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateKey(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Equal(second) {
		t.Error("kunci yang dibaca ulang berbeda")
	}
	if err := os.WriteFile(path, []byte("bukan pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path, false); err == nil {
		t.Error("isi bukan PEM harus error")
	}
}

func TestCSRFValid(t *testing.T) {
	req := func(method, cookie, header string) *http.Request {
		r := httptest.NewRequest(method, "/v1/x", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: cookie})
		}
		if header != "" {
			r.Header.Set(HeaderCSRF, header)
		}
		return r
	}
	cases := []struct {
		name string
		r    *http.Request
		want bool
	}{
		{"GET tanpa token", req(http.MethodGet, "", ""), true},
		{"POST cocok", req(http.MethodPost, "a", "a"), true},
		{"POST beda", req(http.MethodPost, "a", "b"), false},
		{"POST tanpa cookie", req(http.MethodPost, "", "a"), false},
		{"POST tanpa header", req(http.MethodPost, "a", ""), false},
		{"PATCH tanpa keduanya", req(http.MethodPatch, "", ""), false},
		{"DELETE cocok", req(http.MethodDelete, "x", "x"), true},
	}
	for _, c := range cases {
		if got := CSRFValid(c.r); got != c.want {
			t.Errorf("%s: dapat %v", c.name, got)
		}
	}
}

func TestMFATokenIsSeparateFromAccessToken(t *testing.T) {
	s := newSigner(t)
	userID := uuid.New()
	mfa, err := s.SignMFA(userID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.VerifyMFA(mfa); err != nil || got != userID {
		t.Errorf("VerifyMFA: %v, %v", got, err)
	}
	if _, err := s.Verify(mfa); err == nil {
		t.Error("token MFA tidak boleh diterima sebagai access token")
	}
	access, _ := s.Sign(userID, uuid.New(), uuid.New(), "sdm", time.Now())
	if _, err := s.VerifyMFA(access); err == nil {
		t.Error("access token tidak boleh diterima sebagai token MFA")
	}
	expired, _ := s.SignMFA(userID, time.Now().Add(-time.Hour))
	if _, err := s.VerifyMFA(expired); err == nil {
		t.Error("token MFA kedaluwarsa diterima")
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		password, email string
		ok              bool
	}{
		{"pendek", "", false},
		{"pejuang123", "", false},
		{"PEJUANG123", "", false},
		{"sdm@pejuang.id", "sdm@pejuang.id", false},
		{strings.Repeat("a", 257), "", false},
		{"kuda-terbang-biru-7", "sdm@pejuang.id", true},
	}
	for _, c := range cases {
		if got := ValidatePassword(c.password, c.email) == ""; got != c.ok {
			t.Errorf("%q: lolos=%v", c.password, got)
		}
	}
}

func TestHashTokenIsNotPlain(t *testing.T) {
	token, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 43 {
		t.Errorf("token terlalu pendek: %d", len(token))
	}
	if HashToken(token) == token || HashToken(token) != HashToken(token) {
		t.Error("hash token harus deterministik dan berbeda dari token")
	}
}
