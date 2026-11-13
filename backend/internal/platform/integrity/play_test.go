package integrity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPlayVerifier(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	hash := strings.Repeat("A", 43)
	valid := func() playPayload {
		var p playPayload
		p.TokenPayloadExternal.RequestDetails.PackageName = "id.pejuang.app"
		p.TokenPayloadExternal.RequestDetails.RequestHash = hash
		p.TokenPayloadExternal.RequestDetails.TimestampMillis = "1800000000000"
		p.TokenPayloadExternal.AppIntegrity.PackageName = "id.pejuang.app"
		p.TokenPayloadExternal.AppIntegrity.Recognition = "PLAY_RECOGNIZED"
		p.TokenPayloadExternal.DeviceIntegrity.Recognition = []string{"MEETS_DEVICE_INTEGRITY"}
		return p
	}
	tests := []struct {
		name  string
		alter func(*playPayload)
		want  error
	}{
		{"valid", func(*playPayload) {}, nil},
		{"wrong request package", func(p *playPayload) { p.TokenPayloadExternal.RequestDetails.PackageName = "other" }, ErrInvalid},
		{"wrong app package", func(p *playPayload) { p.TokenPayloadExternal.AppIntegrity.PackageName = "other" }, ErrInvalid},
		{"wrong hash", func(p *playPayload) { p.TokenPayloadExternal.RequestDetails.RequestHash = strings.Repeat("B", 43) }, ErrInvalid},
		{"unrecognized app", func(p *playPayload) { p.TokenPayloadExternal.AppIntegrity.Recognition = "UNRECOGNIZED_VERSION" }, ErrInvalid},
		{"basic only", func(p *playPayload) {
			p.TokenPayloadExternal.DeviceIntegrity.Recognition = []string{"MEETS_BASIC_INTEGRITY"}
		}, ErrBasicOnly},
		{"no verdict", func(p *playPayload) { p.TokenPayloadExternal.DeviceIntegrity.Recognition = nil }, ErrInvalid},
		{"expired", func(p *playPayload) { p.TokenPayloadExternal.RequestDetails.TimestampMillis = "1799999909999" }, ErrInvalid},
		{"future", func(p *playPayload) { p.TokenPayloadExternal.RequestDetails.TimestampMillis = "1800000005001" }, ErrInvalid},
		{"invalid time", func(p *playPayload) { p.TokenPayloadExternal.RequestDetails.TimestampMillis = "x" }, ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid()
			tt.alter(&p)
			raw, _ := json.Marshal(p)
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://playintegrity.googleapis.com/v1/id.pejuang.app:decodeIntegrityToken" || r.Header.Get("Authorization") != "Bearer server-access" {
					t.Error("request Google tidak sesuai")
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["integrity_token"] != "encrypted-token" {
					t.Error("token harus dikirim ke Google untuk didekode")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw))), Header: make(http.Header)}, nil
			})}
			v := PlayVerifier{PackageName: "id.pejuang.app", AccessToken: func(context.Context) (string, error) { return "server-access", nil }, Client: client, Now: func() time.Time { return now }}
			err := v.Verify(context.Background(), "encrypted-token", hash)
			if !errors.Is(err, tt.want) {
				t.Fatalf("hasil %v, ingin %v", err, tt.want)
			}
		})
	}
}

func TestPlayVerifierServiceFailure(t *testing.T) {
	hash := strings.Repeat("A", 43)
	for _, tc := range []struct {
		status int
		want   error
	}{{400, ErrInvalid}, {401, ErrInvalid}, {429, ErrUnavailable}, {500, ErrUnavailable}, {302, ErrInvalid}} {
		v := PlayVerifier{PackageName: "id.pejuang.app", AccessToken: func(context.Context) (string, error) { return "access", nil }, Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})}}
		if err := v.Verify(context.Background(), "token", hash); !errors.Is(err, tc.want) {
			t.Errorf("status %d: %v", tc.status, err)
		}
	}
	v := PlayVerifier{PackageName: "id.pejuang.app", AccessToken: func(context.Context) (string, error) { return "", errors.New("credentials unavailable") }}
	if err := v.Verify(context.Background(), "token", hash); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestPlayVerifierRejectsMalformedVerdictAndBadInput(t *testing.T) {
	called := false
	v := PlayVerifier{
		PackageName: "id.pejuang.app",
		AccessToken: func(context.Context) (string, error) { called = true; return "access", nil },
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"tokenPayloadExternal":{}} {}`))}, nil
		})},
	}
	if err := v.Verify(context.Background(), "token", "tidak-valid"); !errors.Is(err, ErrInvalid) || called {
		t.Fatalf("hash tidak valid harus ditolak sebelum akses layanan: %v", err)
	}
	if err := v.Verify(context.Background(), "token", strings.Repeat("A", 43)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("JSON lebih dari satu objek harus ditolak: %v", err)
	}
	v.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("network failure") })}
	if err := v.Verify(context.Background(), "token", strings.Repeat("A", 43)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("gangguan jaringan harus dilaporkan: %v", err)
	}
}
