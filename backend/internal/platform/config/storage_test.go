package config

import "testing"

func TestS3StorageConfig(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SIGNING_KEY_PATH", "jwt.pem")
	t.Setenv("FIELD_ENCRYPTION_KEY_PATH", "field.key")
	t.Setenv("FACE_MODELS_DIR", "")
	t.Setenv("MFA_REQUIRED", "true")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("STORAGE_DRIVER", "s3")
	t.Setenv("STORAGE_ENDPOINT", "http://127.0.0.1:8333")
	t.Setenv("STORAGE_BUCKET", "hris-private")
	t.Setenv("STORAGE_ACCESS_KEY", "test-access")
	t.Setenv("STORAGE_SECRET_KEY", "test-secret")
	t.Setenv("SELFIE_DIR", "")
	if cfg, err := Load(); err != nil || cfg.StorageRegion != "us-east-1" {
		t.Fatalf("konfigurasi S3 valid ditolak: %+v, %v", cfg.StorageRegion, err)
	}
	t.Setenv("STORAGE_ENDPOINT", "http://example.com:8333")
	if _, err := Load(); err == nil {
		t.Fatal("HTTP S3 non-loopback diterima")
	}
	t.Setenv("STORAGE_ENDPOINT", "http://127.0.0.1:8333")
	t.Setenv("STORAGE_SECRET_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("S3 tanpa secret key diterima")
	}
}
