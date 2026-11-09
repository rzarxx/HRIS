package integrity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceAccountToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"type": "service_account", "client_email": "test@example.iam.gserviceaccount.com", "token_uri": googleTokenURL, "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))})
	path := filepath.Join(t.TempDir(), "account.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != googleTokenURL || r.Method != http.MethodPost {
			t.Error("URL OAuth salah")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Error("grant salah")
		}
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if len(parts) != 3 {
			t.Fatal("assertion bukan JWT")
		}
		claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims map[string]any
		if err := json.Unmarshal(claimsRaw, &claims); err != nil {
			t.Fatal(err)
		}
		if claims["aud"] != googleTokenURL || claims["scope"] != playScope || claims["iss"] != "test@example.iam.gserviceaccount.com" {
			t.Error("klaim JWT tidak sesuai")
		}
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
			t.Fatalf("tanda tangan JWT: %v", err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"temporary","token_type":"Bearer","expires_in":3600}`))}, nil
	})}
	account, err := LoadServiceAccount(path, client)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		token, err := account.Token(context.Background())
		if err != nil || token != "temporary" {
			t.Fatalf("token: %q, %v", token, err)
		}
	}
	if requests != 1 {
		t.Fatalf("token harus dicache, request %d", requests)
	}
}

func TestServiceAccountRejectsInvalidCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"type":"service_account","token_uri":"https://example.com/token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServiceAccount(path, nil); err == nil {
		t.Fatal("kredensial tidak valid harus ditolak")
	}
}
