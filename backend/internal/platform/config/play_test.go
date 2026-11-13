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
