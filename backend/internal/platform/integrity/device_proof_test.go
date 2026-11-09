package integrity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestDeviceProof(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("A", 43)
	digest := sha256.Sum256([]byte("HRIS-DEVICE-PROOF-V1\n" + hash))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(sig)
	if err := VerifyDeviceProof(der, hash, encoded); err != nil {
		t.Fatal(err)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherDER, err := x509.MarshalPKIXPublicKey(&other.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key             []byte
		hash, signature string
	}{
		{otherDER, hash, encoded}, {der, strings.Repeat("A", 42) + "E", encoded},
		{der, hash, "bukan-tanda-tangan"}, {[]byte("bad"), hash, encoded},
		{der, hash, encoded + "="}, {der, "bad", encoded},
	} {
		if err := VerifyDeviceProof(tc.key, tc.hash, tc.signature); !errors.Is(err, ErrInvalid) {
			t.Fatalf("bukti tidak valid diterima: %v", err)
		}
	}
}
