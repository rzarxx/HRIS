// Package config membaca konfigurasi dari environment (MASTER.md Lampiran B).
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	DatabaseURL           string
	MigrateDatabaseURL    string
	JWTSigningKeyPath     string
	JWTKeyID              string
	CookieDomain          string
	CORSAllowedOrigins    []string
	LogLevel              string
	SeedSDMPassword       string
	DevSimulateAttendance bool
	// DevAllowUnverifiedAttendance mengizinkan presensi tanpa verifikasi perangkat dan wajah (D36).
	DevAllowUnverifiedAttendance bool
	// ChallengeTTL adalah umur tantangan presensi. Nol berarti memakai bawaan modul presensi.
	ChallengeTTL           time.Duration
	FieldEncryptionKeyPath string
	// FaceModelsDir berisi yunet.onnx dan sface.onnx untuk menghitung wajah dari selfie (D43), dan
	// OnnxRuntimeLib adalah pustaka ONNX Runtime. Kosong berarti verifikasi wajah tidak tersedia.
	// SelfieDir adalah folder selfie terenkripsi.
	FaceModelsDir            string
	OnnxRuntimeLib           string
	SelfieDir                string
	PlayIntegrityPackage     string
	PlayIntegrityCredentials string
	MFARequired              bool
	// CookieSecure menandai cookie hanya untuk HTTPS. Bawaan: aktif di luar development.
	CookieSecure bool
	// TrustProxy memercayai X-Real-IP dari reverse proxy di mesin yang sama.
	TrustProxy bool
}

func (c Config) IsDevelopment() bool { return c.AppEnv == "development" }

// LoadDotEnv memuat file .env bila ada; hanya untuk pengembangan lokal.
// Variabel yang sudah ada di environment tidak ditimpa.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return s.Err()
}

// Load gagal bila secret wajib kosong (MASTER.md §11.4).
func Load() (Config, error) {
	cfg := Config{
		AppEnv:                   env("APP_ENV", "production"),
		HTTPAddr:                 env("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		MigrateDatabaseURL:       os.Getenv("MIGRATE_DATABASE_URL"),
		JWTSigningKeyPath:        os.Getenv("JWT_SIGNING_KEY_PATH"),
		JWTKeyID:                 env("JWT_KEY_ID", "dev-1"),
		CookieDomain:             os.Getenv("COOKIE_DOMAIN"),
		LogLevel:                 env("LOG_LEVEL", "info"),
		SeedSDMPassword:          os.Getenv("SEED_SDM_PASSWORD"),
		FieldEncryptionKeyPath:   os.Getenv("FIELD_ENCRYPTION_KEY_PATH"),
		FaceModelsDir:            os.Getenv("FACE_MODELS_DIR"),
		OnnxRuntimeLib:           os.Getenv("ONNXRUNTIME_LIB"),
		SelfieDir:                os.Getenv("SELFIE_DIR"),
		PlayIntegrityPackage:     os.Getenv("PLAY_INTEGRITY_PACKAGE"),
		PlayIntegrityCredentials: os.Getenv("PLAY_INTEGRITY_CREDENTIALS"),
		MFARequired:              true,
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, origin)
		}
	}
	if raw := os.Getenv("DEV_SIMULATE_ATTENDANCE"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("DEV_SIMULATE_ATTENDANCE: %w", err)
		}
		cfg.DevSimulateAttendance = v
	}
	if raw := os.Getenv("DEV_ALLOW_UNVERIFIED_ATTENDANCE"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("DEV_ALLOW_UNVERIFIED_ATTENDANCE: %w", err)
		}
		cfg.DevAllowUnverifiedAttendance = v
	}
	if raw := os.Getenv("CHALLENGE_TTL_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return cfg, errors.New("CHALLENGE_TTL_SECONDS harus bilangan bulat positif")
		}
		cfg.ChallengeTTL = time.Duration(seconds) * time.Second
	}
	if raw := os.Getenv("MFA_REQUIRED"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("MFA_REQUIRED: %w", err)
		}
		cfg.MFARequired = v
	}
	cfg.CookieSecure = !cfg.IsDevelopment()
	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = v
	}
	if !cfg.CookieSecure && !cfg.IsDevelopment() {
		return cfg, errors.New("COOKIE_SECURE=false hanya boleh di APP_ENV=development")
	}
	if raw := os.Getenv("TRUST_PROXY"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("TRUST_PROXY: %w", err)
		}
		cfg.TrustProxy = v
	}
	if !cfg.MFARequired && !cfg.IsDevelopment() {
		return cfg, errors.New("MFA_REQUIRED=false hanya boleh di APP_ENV=development")
	}
	if cfg.FieldEncryptionKeyPath == "" {
		return cfg, errors.New("FIELD_ENCRYPTION_KEY_PATH wajib diisi")
	}
	if cfg.FaceModelsDir != "" && (cfg.SelfieDir == "" || cfg.OnnxRuntimeLib == "") {
		return cfg, errors.New("SELFIE_DIR dan ONNXRUNTIME_LIB wajib diisi bila FACE_MODELS_DIR diisi")
	}
	if (cfg.PlayIntegrityPackage == "") != (cfg.PlayIntegrityCredentials == "") {
		return cfg, errors.New("PLAY_INTEGRITY_PACKAGE dan PLAY_INTEGRITY_CREDENTIALS harus diisi bersama")
	}
	if cfg.PlayIntegrityPackage != "" && cfg.DevAllowUnverifiedAttendance {
		return cfg, errors.New("Play Integrity tidak dapat digabung dengan DEV_ALLOW_UNVERIFIED_ATTENDANCE")
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL wajib diisi")
	}
	if cfg.JWTSigningKeyPath == "" {
		return cfg, errors.New("JWT_SIGNING_KEY_PATH wajib diisi")
	}
	if cfg.DevAllowUnverifiedAttendance && !cfg.IsDevelopment() {
		return cfg, errors.New("DEV_ALLOW_UNVERIFIED_ATTENDANCE hanya boleh di APP_ENV=development")
	}
	if cfg.DevSimulateAttendance && !cfg.IsDevelopment() {
		return cfg, errors.New("DEV_SIMULATE_ATTENDANCE hanya boleh di APP_ENV=development")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
