package attendance

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

type stubPlayVerifier struct {
	err error
}

func (s stubPlayVerifier) Verify(context.Context, string, string) error {
	return s.err
}

type stubAttestationVerifier struct {
	err error
}

func (s stubAttestationVerifier) Verify([]string, []byte, string) error {
	return s.err
}

func generateTestDeviceKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return priv, der
}

func signTestDeviceProof(t *testing.T, priv *ecdsa.PrivateKey, requestHash string) string {
	t.Helper()
	digest := sha256.Sum256([]byte("HRIS-DEVICE-PROOF-V1\n" + requestHash))
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(sig)
}

func sampleRequestHash(t *testing.T, now time.Time) string {
	t.Helper()
	h, err := integrity.AttendanceHash(
		strings.Repeat("a", 64),
		-6.2001,
		106.8166,
		now,
		strings.Repeat("b", 64),
		strings.Repeat("c", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHardSignalsBlockAllAttackVectors(t *testing.T) {
	cases := []struct {
		name    string
		signals Signals
		want    string
	}{
		{"fake GPS mock location", Signals{IsMockLocation: true}, "mock_location"},
		{"fake GPS package installed", Signals{FakeGPSAppDetected: true}, "fake_gps_app"},
		{"rooted device", Signals{RootDetected: true}, "root_detected"},
		{"emulator", Signals{EmulatorDetected: true}, "emulator"},
	}
	for _, tc := range cases {
		flags := tc.signals.hardFlags()
		if !slices.Contains(flags, tc.want) {
			t.Errorf("%s: flags=%v, ingin %s", tc.name, flags, tc.want)
		}
	}
}

func TestSoftSignalWeightsMatchMasterSpec(t *testing.T) {
	cases := map[string]int{
		flagAccuracyLow:          15,
		flagLocationJump:         40,
		flagClockSkew:            20,
		flagNewDevice:            10,
		flagFaceBorderline:       20,
		flagLivenessRetry:        25,
		flagIntegrityBasicOnly:   30,
		flagIntegrityManual:      30,
		flagIntegrityUnavailable: 30,
	}
	for flag, want := range cases {
		if got := RiskScore([]string{flag}); got != want {
			t.Errorf("RiskScore(%s) = %d, ingin %d", flag, got, want)
		}
	}
	if got := RiskScore([]string{flagIntegrityManual, flagLocationJump}); got < riskRejectMin {
		t.Errorf("manual (+30) + location_jump (+40) = %d, harus >= %d (ditolak)", got, riskRejectMin)
	}
}

func TestVerifyDeviceIntegrityTiersAndAttacks(t *testing.T) {
	now := time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)
	hash := sampleRequestHash(t, now)
	priv, pubDER := generateTestDeviceKey(t)
	otherPriv, _ := generateTestDeviceKey(t)
	validProof := signTestDeviceProof(t, priv, hash)
	wrongKeyProof := signTestDeviceProof(t, otherPriv, hash)

	t.Run("Jalur A play: valid token + valid device_proof passes", func(t *testing.T) {
		svc := NewService(nil, Options{
			Now:              func() time.Time { return now },
			PlayVerifier:     stubPlayVerifier{err: nil},
			RequireDeviceKey: true,
		})
		dev := store.Device{IntegrityTier: store.DeviceIntegrityTierPlay, PublicKey: pubDER}
		flags, reason, ok := svc.verifyDeviceIntegrity(context.Background(), dev, hash, "play-tok", validProof, nil, true)
		if !ok || reason != "" || len(flags) != 0 {
			t.Fatalf("ingin lolos bersih, dapat ok=%v reason=%q flags=%v", ok, reason, flags)
		}
	})

	t.Run("Jalur A play: rejects stolen Play token signed by different device key", func(t *testing.T) {
		svc := NewService(nil, Options{
			Now:              func() time.Time { return now },
			PlayVerifier:     stubPlayVerifier{err: nil},
			RequireDeviceKey: true,
		})
		dev := store.Device{IntegrityTier: store.DeviceIntegrityTierPlay, PublicKey: pubDER}
		_, reason, ok := svc.verifyDeviceIntegrity(context.Background(), dev, hash, "play-tok", wrongKeyProof, nil, true)
		if ok || reason != flagIntegrityFailed {
			t.Fatalf("token Play dengan kunci perangkat berbeda harus ditolak: ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("Jalur A play: nonce/request_hash mismatch logs integrity_nonce_mismatch", func(t *testing.T) {
		svc := NewService(nil, Options{
			Now:          func() time.Time { return now },
			PlayVerifier: stubPlayVerifier{err: integrity.ErrNonceMismatch},
		})
		dev := store.Device{IntegrityTier: store.DeviceIntegrityTierPlay, PublicKey: pubDER}
		_, reason, ok := svc.verifyDeviceIntegrity(context.Background(), dev, hash, "play-tok", validProof, nil, true)
		if ok || reason != flagIntegrityNonceMismatch {
			t.Fatalf("ingin integrity_nonce_mismatch, dapat ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("Jalur A play: service unavailable enters review on clock but blocks face enroll", func(t *testing.T) {
		svc := NewService(nil, Options{
			Now:          func() time.Time { return now },
			PlayVerifier: stubPlayVerifier{err: integrity.ErrUnavailable},
		})
		dev := store.Device{IntegrityTier: store.DeviceIntegrityTierPlay, PublicKey: pubDER}

		flags, reason, ok := svc.verifyDeviceIntegrity(context.Background(), dev, hash, "play-tok", validProof, nil, true)
		if !ok || reason != "" || !slices.Contains(flags, flagIntegrityUnavailable) || RiskScore(flags) != 30 {
			t.Fatalf("clock-in saat layanan Play mati harus masuk review (+30): ok=%v flags=%v", ok, flags)
		}

		_, reason, ok = svc.verifyDeviceIntegrity(context.Background(), dev, hash, "play-tok", validProof, nil, false)
		if ok || reason != flagIntegrityFailed {
			t.Fatalf("pendaftaran wajah saat layanan Play mati harus ditolak: ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("Jalur B attestation: valid hardware chain + device_proof passes, failure never downgrades", func(t *testing.T) {
		svc := NewService(nil, Options{
			Now:                 func() time.Time { return now },
			AttestationVerifier: stubAttestationVerifier{err: nil},
		})
		dev := store.Device{IntegrityTier: store.DeviceIntegrityTierAttestation, PublicKey: pubDER}
		flags, reason, ok := svc.verifyDeviceIntegrity(context.Background(), dev, hash, "", validProof, []string{"leaf", "root"}, true)
		if !ok || reason != "" || len(flags) != 0 {
			t.Fatalf("Jalur B valid harus lolos: ok=%v reason=%q flags=%v", ok, reason, flags)
		}

		svcFail := NewService(nil, Options{
			Now:                 func() time.Time { return now },
			AttestationVerifier: stubAttestationVerifier{err: errors.New("invalid chain")},
		})
		_, reason, ok = svcFail.verifyDeviceIntegrity(context.Background(), dev, hash, "", validProof, []string{"leaf", "root"}, true)
		if ok || reason != flagIntegrityFailed {
			t.Fatalf("Jalur B gagal harus ditolak tanpa turun jalur: ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("Jalur D manual: always adds integrity_manual (+30 -> review) and rejects when expired", func(t *testing.T) {
		svc := NewService(nil, Options{Now: func() time.Time { return now }})
		activeDev := store.Device{
			IntegrityTier:       store.DeviceIntegrityTierManual,
			ManualApprovedUntil: pgconv.Time(now.Add(24 * time.Hour)),
			PublicKey:           pubDER,
		}
		flags, reason, ok := svc.verifyDeviceIntegrity(context.Background(), activeDev, hash, "", validProof, nil, true)
		if !ok || reason != "" || !slices.Contains(flags, flagIntegrityManual) || RiskScore(flags) != 30 {
			t.Fatalf("Jalur D aktif harus selalu menghasilkan integrity_manual (+30): ok=%v flags=%v", ok, flags)
		}

		expiredDev := store.Device{
			IntegrityTier:       store.DeviceIntegrityTierManual,
			ManualApprovedUntil: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			PublicKey:           pubDER,
		}
		_, reason, ok = svc.verifyDeviceIntegrity(context.Background(), expiredDev, hash, "", validProof, nil, true)
		if ok || reason != flagIntegrityFailed {
			t.Fatalf("Jalur D kedaluwarsa harus ditolak: ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("Jalur E kiosk: requires bound device key and valid signature", func(t *testing.T) {
		svc := NewService(nil, Options{Now: func() time.Time { return now }})
		kioskDev := store.Device{IntegrityTier: store.DeviceIntegrityTierKiosk, PublicKey: pubDER}
		flags, reason, ok := svc.verifyDeviceIntegrity(context.Background(), kioskDev, hash, "", validProof, nil, true)
		if !ok || reason != "" || len(flags) != 0 {
			t.Fatalf("Jalur E kiosk dengan bukti kunci valid harus lolos: ok=%v reason=%q", ok, reason)
		}

		unboundKiosk := store.Device{IntegrityTier: store.DeviceIntegrityTierKiosk}
		_, reason, ok = svc.verifyDeviceIntegrity(context.Background(), unboundKiosk, hash, "", "", nil, true)
		if ok || reason != flagIntegrityFailed {
			t.Fatalf("Jalur E kiosk tanpa kunci terikat harus ditolak: ok=%v reason=%q", ok, reason)
		}
	})
}
