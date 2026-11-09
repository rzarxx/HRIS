// Command purge-selfies menghapus selfie setelah 90 hari; jalankan harian oleh scheduler host.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gagal:", err)
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
	if cfg.SelfieDir == "" {
		return errors.New("SELFIE_DIR wajib diisi")
	}
	info, err := os.Stat(cfg.SelfieDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("SELFIE_DIR harus berupa direktori")
	}
	cipher, err := fieldcrypt.LoadOrCreate(cfg.FieldEncryptionKeyPath, false)
	if err != nil {
		return err
	}
	selfies, err := attendance.NewDiskStore(cfg.SelfieDir, cipher)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	n, err := attendance.PurgeExpiredSelfies(ctx, pool, selfies, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("selfie dihapus: %d\n", n)
	return nil
}
