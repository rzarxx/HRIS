package device

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/store"
)

func testP256KeyB64(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestRegisterInputPublicKeyValidation(t *testing.T) {
	hash := strings.Repeat("a", 64)
	validKey := testP256KeyB64(t)
	in := RegisterInput{DeviceHash: hash, PublicKey: &validKey}
	if err := in.validate(); err != nil {
		t.Fatalf("kunci P-256 valid ditolak: %v", err)
	}
	der, ok := in.decodePublicKey()
	if !ok || len(der) == 0 {
		t.Fatal("decodePublicKey gagal")
	}

	for _, bad := range []string{"bukan-base64!", base64.StdEncoding.EncodeToString([]byte("bukan-der"))} {
		b := bad
		invalid := RegisterInput{DeviceHash: hash, PublicKey: &b}
		if err := invalid.validate(); err == nil {
			t.Errorf("kunci rusak %q seharusnya ditolak", bad)
		}
	}
}

func TestValidateTierInput(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	in30Days := now.AddDate(0, 0, 30).Format(time.RFC3339)
	in91Days := now.AddDate(0, 0, 91).Format(time.RFC3339)
	past := now.Add(-time.Hour).Format(time.RFC3339)

	for _, tier := range []string{"play", "attestation", "kiosk"} {
		got, manualUntil, reason, err := validateTierInput(TierInput{
			IntegrityTier: tier,
			Reason:        "Perubahan jalur oleh SDM",
		}, now)
		if err != nil || string(got) != tier || manualUntil.Valid || reason == "" {
			t.Fatalf("tier %s gagal: got=%s valid=%v err=%v", tier, got, manualUntil.Valid, err)
		}
	}

	got, manualUntil, _, err := validateTierInput(TierInput{
		IntegrityTier:       "manual",
		ManualApprovedUntil: &in30Days,
		Reason:              "HP tidak punya GMS",
	}, now)
	if err != nil || got != store.DeviceIntegrityTierManual || !manualUntil.Valid {
		t.Fatalf("jalur manual 30 hari gagal: %v", err)
	}

	for name, bad := range map[string]TierInput{
		"tier tidak dikenal": {IntegrityTier: "root", Reason: "alasan"},
		"alasan terlalu pdk": {IntegrityTier: "play", Reason: "ab"},
		"manual tanpa batas": {IntegrityTier: "manual", Reason: "alasan valid"},
		"manual sudah lewat": {IntegrityTier: "manual", ManualApprovedUntil: &past, Reason: "alasan valid"},
		"manual > 90 hari":   {IntegrityTier: "manual", ManualApprovedUntil: &in91Days, Reason: "alasan valid"},
	} {
		if _, _, _, err := validateTierInput(bad, now); err == nil {
			t.Errorf("%s seharusnya ditolak", name)
		}
	}
}
