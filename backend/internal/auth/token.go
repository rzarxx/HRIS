package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	Issuer      = "hris-api"
	AudienceWeb = "hris-web"
	AudienceMFA = "hris-mfa"
	AccessTTL   = 15 * time.Minute
	MFATokenTTL = 5 * time.Minute
)

// Claims mengikuti MASTER.md §5.2.
type Claims struct {
	EID  string `json:"eid"`
	Role string `json:"role"`
	SID  string `json:"sid"`
	jwt.RegisteredClaims
}

// TokenSigner menandatangani dan memverifikasi JWT EdDSA.
type TokenSigner struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	keyID   string
}

func NewTokenSigner(private ed25519.PrivateKey, keyID string) *TokenSigner {
	return &TokenSigner{private: private, public: private.Public().(ed25519.PublicKey), keyID: keyID}
}

// LoadOrCreateKey membaca kunci Ed25519 (PKCS#8 PEM). Hanya pengembangan yang boleh
// membuat kunci baru; lingkungan lain gagal start bila kunci tidak ada.
func LoadOrCreateKey(path string, allowCreate bool) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowCreate {
		return createKey(path)
	}
	if err != nil {
		return nil, fmt.Errorf("baca kunci JWT: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("kunci JWT bukan PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse kunci JWT: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("kunci JWT bukan Ed25519")
	}
	return key, nil
}

func createKey(path string) (ed25519.PrivateKey, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return private, nil
}

func (s *TokenSigner) Sign(userID, employeeID, sessionID uuid.UUID, role string, now time.Time) (string, error) {
	claims := Claims{
		EID:  employeeID.String(),
		Role: role,
		SID:  sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{AudienceWeb},
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.keyID
	return token.SignedString(s.private)
}

// Verify mengunci algoritma ke EdDSA sehingga token alg lain ditolak.
func (s *TokenSigner) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.public, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(AudienceWeb),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// SignMFA menerbitkan token sementara antara langkah sandi dan kode TOTP (F4).
// Audience berbeda sehingga tidak bisa dipakai sebagai access token.
func (s *TokenSigner) SignMFA(userID uuid.UUID, now time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{AudienceMFA},
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(MFATokenTTL)),
		ID:        uuid.NewString(),
	})
	token.Header["kid"] = s.keyID
	return token.SignedString(s.private)
}

// VerifyMFA mengembalikan user ID dari token sementara.
func (s *TokenSigner) VerifyMFA(raw string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.public, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(AudienceMFA),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(claims.Subject)
}

// NewOpaqueToken membuat token acak 256-bit untuk refresh token dan CSRF.
func NewOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken menyimpan refresh token sebagai hash; token asli tidak pernah masuk database.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
