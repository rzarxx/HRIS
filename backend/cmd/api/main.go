// Command api menjalankan HTTP API HRIS Pejuang.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/announcement"
	"github.com/rzarxx/HRIS/backend/internal/approval"
	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/dashboard"
	"github.com/rzarxx/HRIS/backend/internal/device"
	"github.com/rzarxx/HRIS/backend/internal/devsim"
	"github.com/rzarxx/HRIS/backend/internal/employee"
	"github.com/rzarxx/HRIS/backend/internal/face"
	"github.com/rzarxx/HRIS/backend/internal/leave"
	"github.com/rzarxx/HRIS/backend/internal/location"
	"github.com/rzarxx/HRIS/backend/internal/org"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
	"github.com/rzarxx/HRIS/backend/internal/report"
	"github.com/rzarxx/HRIS/backend/internal/server"
	"github.com/rzarxx/HRIS/backend/internal/settings"
	"github.com/rzarxx/HRIS/backend/internal/shift"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("gagal menjalankan API", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	key, err := auth.LoadOrCreateKey(cfg.JWTSigningKeyPath, cfg.IsDevelopment())
	if err != nil {
		return err
	}
	signer := auth.NewTokenSigner(key, cfg.JWTKeyID)
	fieldCipher, err := fieldcrypt.LoadOrCreate(cfg.FieldEncryptionKeyPath, cfg.IsDevelopment())
	if err != nil {
		return err
	}
	enrollGeofence := cfg.FaceEnrollRequiresGeofence
	attendanceOpt := attendance.Options{
		AllowUnverified:          cfg.DevAllowUnverifiedAttendance,
		RequireDeviceKey:         !cfg.DevAllowUnverifiedAttendance,
		ChallengeTTL:             cfg.ChallengeTTL,
		Cipher:                   fieldCipher,
		FaceThresholdPass:        cfg.FaceThresholdPass,
		FaceThresholdReview:      cfg.FaceThresholdReview,
		FaceEnrollMinSimilarity:  cfg.FaceEnrollMinSimilarity,
		EnrollRequiresGeofence:   &enrollGeofence,
		FaceStraightTurnMax:      cfg.FaceStraightTurnMax,
		FaceGestureTurnMin:       cfg.FaceGestureTurnMin,
		FaceGestureTurnDelta:     cfg.FaceGestureTurnDelta,
		FaceGestureMinSimilarity: cfg.FaceGestureMinSimilarity,
		RiskReviewMin:            cfg.RiskReviewMin,
		RiskRejectMin:            cfg.RiskRejectMin,
	}
	if cfg.PlayIntegrityPackage != "" {
		account, err := integrity.LoadServiceAccount(cfg.PlayIntegrityCredentials, nil)
		if err != nil {
			return err
		}
		attendanceOpt.PlayVerifier = integrity.PlayVerifier{PackageName: cfg.PlayIntegrityPackage, AccessToken: account.Token}
	}
	if cfg.KeyAttestationRootCAPath != "" {
		roots, err := integrity.LoadAttestationRoots(cfg.KeyAttestationRootCAPath)
		if err != nil {
			return err
		}
		var certDigest []byte
		if cfg.KeyAttestationCertSHA256 != "" {
			certDigest, err = integrity.ParseAttestationCertSHA256(cfg.KeyAttestationCertSHA256)
			if err != nil {
				return err
			}
		}
		attendanceOpt.AttestationVerifier = integrity.KeyAttestationVerifier{
			PackageName:       cfg.KeyAttestationPackage,
			SigningCertSHA256: certDigest,
			Roots:             roots,
		}
	}
	if cfg.SelfieDir != "" {
		selfies, err := attendance.NewDiskStore(cfg.SelfieDir, fieldCipher)
		if err != nil {
			return err
		}
		attendanceOpt.Selfies = selfies
	}
	if cfg.FaceModelsDir != "" {
		engine, err := face.NewSFace(cfg.FaceModelsDir, cfg.OnnxRuntimeLib)
		if err != nil {
			return err
		}
		attendanceOpt.Faces = engine
		slog.Info("verifikasi wajah aktif", "model", engine.Model())
	} else {
		slog.Warn("FACE_MODELS_DIR kosong: daftar wajah dan presensi dengan selfie dijawab FACE_SERVICE_UNAVAILABLE")
	}
	authSvc, err := auth.NewService(pool, signer, fieldCipher, cfg.MFARequired)
	if err != nil {
		return err
	}
	authSvc.SetDeviceBinder(device.BindForLogin)
	q := store.New(pool)
	secure := cfg.CookieSecure
	limiter := auth.NewMemoryLimiter()
	approvalSvc := approval.NewService(pool)
	leaveSvc := leave.NewService(pool, authSvc.ResolveScope)
	approvalSvc.Register(store.RequestKindLeave, leaveSvc)

	handler := server.New(server.Deps{
		Auth:         auth.NewHandler(authSvc, secure, cfg.CookieDomain, limiter),
		Org:          org.NewHandler(org.NewService(pool)),
		Employee:     employee.NewHandler(employee.NewService(pool, authSvc.ResolveScope)),
		Announcement: announcement.NewHandler(announcement.NewService(pool)),
		Settings:     settings.NewHandler(settings.NewService(pool)),
		Dashboard:    dashboard.NewHandler(q),
		Report:       report.NewHandler(report.NewService(q, authSvc.ResolveScope)),
		Location:     location.NewHandler(location.NewService(pool)),
		Device:       device.NewHandler(device.NewService(pool)),
		Attendance:   attendance.NewHandler(attendance.NewService(pool, attendanceOpt)),
		Shift:        shift.NewHandler(shift.NewService(pool)),
		Approval:     approval.NewHandler(approvalSvc),
		Leave:        leave.NewHandler(leaveSvc),
		Signer:       signer,
		SessionCheck: authSvc.SessionState,
		Limiter:      limiter,
		Ready:        pool.Ping,
		Options:      server.Options{AllowedOrigins: cfg.CORSAllowedOrigins, Secure: secure, TrustProxy: cfg.TrustProxy},
	})

	if cfg.DevSimulateAttendance {
		go simulateAttendance(ctx, q)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("API berjalan", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

// simulateAttendance menjaga data presensi dummy tetap terisi sampai hari ini.
func simulateAttendance(ctx context.Context, q *store.Queries) {
	fill := func() {
		n, err := devsim.FillAttendance(ctx, q, time.Now())
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				slog.Error("simulasi presensi", "error", err.Error())
			}
			return
		}
		if n > 0 {
			slog.Info("simulasi presensi", "baris_baru", n)
		}
	}
	fill()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fill()
		}
	}
}

func logLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
