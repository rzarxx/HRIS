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
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/dashboard"
	"github.com/rzarxx/HRIS/backend/internal/devsim"
	"github.com/rzarxx/HRIS/backend/internal/employee"
	"github.com/rzarxx/HRIS/backend/internal/org"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/report"
	"github.com/rzarxx/HRIS/backend/internal/server"
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
	authSvc, err := auth.NewService(pool, signer, fieldCipher, cfg.MFARequired)
	if err != nil {
		return err
	}
	q := store.New(pool)
	secure := cfg.CookieSecure
	limiter := auth.NewMemoryLimiter()

	handler := server.New(server.Deps{
		Auth:         auth.NewHandler(authSvc, secure, cfg.CookieDomain, limiter),
		Org:          org.NewHandler(org.NewService(pool)),
		Employee:     employee.NewHandler(employee.NewService(pool, authSvc.ResolveScope)),
		Announcement: announcement.NewHandler(announcement.NewService(pool)),
		Dashboard:    dashboard.NewHandler(q),
		Report:       report.NewHandler(report.NewService(q, authSvc.ResolveScope)),
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
