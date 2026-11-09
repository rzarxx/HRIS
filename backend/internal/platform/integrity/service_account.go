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
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const googleTokenURL = "https://oauth2.googleapis.com/token"
const playScope = "https://www.googleapis.com/auth/playintegrity"

// ServiceAccount adalah sumber token OAuth server. Berkas kunci harus disimpan privat di server.
type ServiceAccount struct {
	email   string
	key     *rsa.PrivateKey
	client  *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
}

// LoadServiceAccount hanya menerima kredensial service account dan tidak membuat kunci baru.
func LoadServiceAccount(path string, client *http.Client) (*ServiceAccount, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("tidak dapat membuka kredensial Google")
	}
	defer f.Close()
	var creds struct {
		Type     string `json:"type"`
		Email    string `json:"client_email"`
		Key      string `json:"private_key"`
		TokenURI string `json:"token_uri"`
	}
	if err := json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&creds); err != nil {
		return nil, errors.New("format kredensial Google tidak valid")
	}
	if creds.Type != "service_account" || creds.Email == "" || strings.ContainsAny(creds.Email, "\r\n") || creds.TokenURI != googleTokenURL {
		return nil, errors.New("kredensial service account tidak valid")
	}
	block, _ := pem.Decode([]byte(creds.Key))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("kunci service account tidak valid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("kunci service account tidak valid")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, errors.New("kunci service account tidak valid")
	}
	if client == nil {
		client = &http.Client{}
	}
	return &ServiceAccount{email: creds.Email, key: key, client: client}, nil
}

// Token cache token dan memperbaruinya sebelum kedaluwarsa. Gagal tertutup bila OAuth gagal.
func (s *ServiceAccount) Token(ctx context.Context) (string, error) {
	if s == nil || s.key == nil || s.client == nil || s.email == "" {
		return "", ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if s.token != "" && now.Add(time.Minute).Before(s.expires) {
		return s.token, nil
	}
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{"iss": s.email, "scope": playScope, "aud": googleTokenURL, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	if err != nil {
		return "", ErrUnavailable
	}
	unsigned := header + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", ErrUnavailable
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {unsigned + "." + encode(signature)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := *s.client
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return "", ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", ErrUnavailable
	}
	var reply struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&reply); err != nil || reply.AccessToken == "" || !strings.EqualFold(reply.TokenType, "Bearer") || reply.ExpiresIn <= 60 || reply.ExpiresIn > 86400 {
		return "", ErrUnavailable
	}
	s.token, s.expires = reply.AccessToken, now.Add(time.Duration(reply.ExpiresIn)*time.Second)
	return s.token, nil
}
