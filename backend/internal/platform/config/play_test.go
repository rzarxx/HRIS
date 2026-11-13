package config

import (
	"testing"
)

func TestPlayIntegrityConfig(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SIGNING_KEY_PATH", "jwt.pem")
	t.Setenv("FIELD_ENCRYPTION_KEY_PATH", "field.key")
	t.Setenv("FACE_MODELS_DIR", "")
	t.Setenv("MFA_REQUIRED", "true")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("DEV_SIMULATE_ATTENDANCE", "false")
	t.Setenv("DEV_ALLOW_UNVERIFIED_ATTENDANCE", "false")
	t.Setenv("PLAY_INTEGRITY_PACKAGE", "id.pejuang.app")
	t.Setenv("PLAY_INTEGRITY_CREDENTIALS", "")
	if _, err := Load(); err == nil {
		t.Fatal("konfigurasi setengah harus ditolak")
	}
	t.Setenv("PLAY_INTEGRITY_CREDENTIALS", "private.json")
	if _, err := Load(); err != nil {
		t.Fatalf("pasangan konfigurasi harus diterima: %v", err)
	}
	t.Setenv("DEV_ALLOW_UNVERIFIED_ATTENDANCE", "true")
	if _, err := Load(); err == nil {
		t.Fatal("bypass tidak boleh dipakai bersama Play Integrity")
	}
	t.Setenv("DEV_ALLOW_UNVERIFIED_ATTENDANCE", "false")
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err != nil {
		t.Fatalf("Play Integrity dengan pengikatan perangkat harus diterima di produksi: %v", err)
	}
}

func TestThresholdAndAttestationConfigValidation(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SIGNING_KEY_PATH", "jwt.pem")
	t.Setenv("FIELD_ENCRYPTION_KEY_PATH", "field.key")
	t.Setenv("FACE_MODELS_DIR", "")
	t.Setenv("MFA_REQUIRED", "true")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("FACE_THRESHOLD_PASS", "0.52")
	t.Setenv("FACE_THRESHOLD_REVIEW", "0.47")
	t.Setenv("FACE_ENROLL_MIN_SIMILARITY", "0.52")
	t.Setenv("RISK_REVIEW_MIN", "35")
	t.Setenv("RISK_REJECT_MIN", "65")
	t.Setenv("KEY_ATTESTATION_CERT_SHA256", "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("konfigurasi ambang valid harus diterima: %v", err)
	}
	if cfg.FaceThresholdPass != 0.52 || cfg.FaceThresholdReview != 0.47 || cfg.RiskReviewMin != 35 || cfg.RiskRejectMin != 65 {
		t.Fatalf("nilai ambang tidak sesuai: %+v", cfg)
	}

	t.Setenv("FACE_THRESHOLD_REVIEW", "0.55")
	if _, err := Load(); err == nil {
		t.Fatal("FACE_THRESHOLD_REVIEW >= FACE_THRESHOLD_PASS harus ditolak")
	}
	t.Setenv("FACE_THRESHOLD_REVIEW", "0.47")

	t.Setenv("RISK_REJECT_MIN", "30")
	if _, err := Load(); err == nil {
		t.Fatal("RISK_REJECT_MIN <= RISK_REVIEW_MIN harus ditolak")
	}
	t.Setenv("RISK_REJECT_MIN", "65")

	t.Setenv("KEY_ATTESTATION_CERT_SHA256", "bukan-hex-64")
	if _, err := Load(); err == nil {
		t.Fatal("KEY_ATTESTATION_CERT_SHA256 tidak valid harus ditolak")
	}
}
