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
	"github.com/rzarxx/HRIS/backend/internal/platform/filestore"
	"github.com/rzarxx/HRIS/backend/internal/storageadmin"
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
	cipher, err := fieldcrypt.LoadOrCreate(cfg.FieldEncryptionKeyPath, false)
	if err != nil {
		return err
	}
	var disk filestore.Store
	if cfg.SelfieDir != "" {
		info, statErr := os.Stat(cfg.SelfieDir)
		if statErr != nil {
			return statErr
		}
		if !info.IsDir() {
			return errors.New("SELFIE_DIR harus berupa direktori")
		}
		disk, err = attendance.NewDiskStore(cfg.SelfieDir, cipher)
		if err != nil {
			return err
		}
	}
	var s3Files *filestore.S3Store
	var initial filestore.S3Config
	if cfg.StorageDriver == "s3" {
		initial = filestore.S3Config{Endpoint: cfg.StorageEndpoint, Bucket: cfg.StorageBucket, Region: cfg.StorageRegion, AccessKey: cfg.StorageAccessKey, SecretKey: cfg.StorageSecretKey}
		s3Files, err = filestore.NewS3Store(initial, cipher)
		if err != nil {
			return err
		}
	}
	var primary filestore.Store = disk
	if s3Files != nil {
		primary = s3Files
	}
	managed := filestore.NewManagedStore(primary, disk, s3Files)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	storageSvc := storageadmin.NewService(pool, cipher, managed, disk, cfg.SelfieDir, initial)
	if err := storageSvc.Refresh(ctx); err != nil {
		return err
	}
	managed.SetRefresh(storageSvc.Refresh)
	if primary == nil && managed.CurrentS3() == nil {
		return nil
	}
	var selfies attendance.SelfieStore = managed
	n, err := attendance.PurgeExpiredSelfies(ctx, pool, selfies, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("selfie dihapus: %d\n", n)
	return nil
}
