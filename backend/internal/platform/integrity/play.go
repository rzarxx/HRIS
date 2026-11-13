// Package integrity memverifikasi verdict integritas yang diperoleh dari layanan tepercaya.
package integrity

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

var ErrInvalid = errors.New("verdict integritas tidak valid")
var ErrNonceMismatch = errors.New("request_hash tidak cocok")
var ErrUnavailable = errors.New("layanan integritas tidak tersedia")

// AccessToken mengambil access token layanan Google dengan scope playintegrity.
// Implementasi produksi harus memperoleh token dari kredensial server, bukan klien aplikasi.
type AccessToken func(context.Context) (string, error)

type PlayVerifier struct {
	PackageName string
	AccessToken AccessToken
	Client      *http.Client
	Now         func() time.Time
}

type playPayload struct {
	TokenPayloadExternal struct {
		RequestDetails struct {
			PackageName     string `json:"requestPackageName"`
			RequestHash     string `json:"requestHash"`
			TimestampMillis string `json:"timestampMillis"`
		} `json:"requestDetails"`
		AppIntegrity struct {
			PackageName string `json:"packageName"`
			Recognition string `json:"appRecognitionVerdict"`
		} `json:"appIntegrity"`
		DeviceIntegrity struct {
			Recognition []string `json:"deviceRecognitionVerdict"`
		} `json:"deviceIntegrity"`
	} `json:"tokenPayloadExternal"`
}

// Verify meminta Google mendekode token; JSON dari klien tidak pernah dipakai sebagai verdict.
// requestHash harus dihitung server dari challenge dan data permintaan, bukan dikirim klien.
func (v PlayVerifier) Verify(ctx context.Context, token, requestHash string) error {
	if v.PackageName == "" || strings.ContainsAny(v.PackageName, "/?#") || v.AccessToken == nil || token == "" || len(token) > 32768 || len(requestHash) != 43 {
		return ErrInvalid
	}
	if hash, err := base64.RawURLEncoding.DecodeString(requestHash); err != nil || len(hash) != 32 {
		return ErrInvalid
	}
	access, err := v.AccessToken(ctx)
	if err != nil || access == "" {
		return ErrUnavailable
	}
	body, err := json.Marshal(map[string]string{"integrity_token": token})
	if err != nil {
		return ErrInvalid
	}
	endpoint := "https://playintegrity.googleapis.com/v1/" + url.PathEscape(v.PackageName) + ":decodeIntegrityToken"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 10 * time.Second}
	if v.Client != nil {
		client = *v.Client
		if client.Timeout == 0 {
			client.Timeout = 10 * time.Second
		}
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return ErrUnavailable
	}
	if res.StatusCode != http.StatusOK {
		return ErrInvalid
	}
	var payload playPayload
	dec := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
	if err := dec.Decode(&payload); err != nil {
		return ErrInvalid
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return ErrInvalid
	}
	details := payload.TokenPayloadExternal.RequestDetails
	app := payload.TokenPayloadExternal.AppIntegrity
	device := payload.TokenPayloadExternal.DeviceIntegrity
	if details.PackageName != v.PackageName || app.PackageName != v.PackageName ||
		app.Recognition != "PLAY_RECOGNIZED" || !slices.Contains(device.Recognition, "MEETS_DEVICE_INTEGRITY") {
		return ErrInvalid
	}
	if subtle.ConstantTimeCompare([]byte(details.RequestHash), []byte(requestHash)) != 1 {
		return fmt.Errorf("%w: %w", ErrNonceMismatch, ErrInvalid)
	}
	ms, err := strconv.ParseInt(details.TimestampMillis, 10, 64)
	if err != nil || ms <= 0 {
		return ErrInvalid
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	age := now.Sub(time.UnixMilli(ms))
	if age < -5*time.Second || age > 90*time.Second {
		return fmt.Errorf("%w: waktu verdict", ErrInvalid)
	}
	return nil
}
