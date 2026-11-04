// Command seed mengisi data dummy untuk pengembangan dan demo.
// Dipakai: go run ./cmd/seed [--reset | --demo]. Ditolak di luar APP_ENV=development.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/devsim"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gagal:", err)
		os.Exit(1)
	}
}

func run() error {
	reset := flag.Bool("reset", false, "kosongkan semua data lalu isi ulang")
	demo := flag.Bool("demo", false, "siapkan atau pulihkan data demo Review Absen, izin kerja, dan perangkat")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	if os.Getenv("APP_ENV") != "development" {
		return errors.New("seed hanya boleh dijalankan dengan APP_ENV=development")
	}
	dsn := os.Getenv("MIGRATE_DATABASE_URL")
	if dsn == "" {
		return errors.New("MIGRATE_DATABASE_URL wajib diisi")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	if *demo {
		if *reset {
			return errors.New("--demo tidak bisa digabung dengan --reset, jalankan terpisah")
		}
		summary, err := devsim.SeedDemo(ctx, pool, time.Now())
		if err != nil {
			return err
		}
		fmt.Printf("Data demo siap untuk tanggal kerja %s: %d absen di Review Absen, %d izin kerja, %d perangkat.\n",
			summary.WorkDate.Format("2006-01-02"), summary.Reviews, summary.Arrangements, summary.Devices)
		return nil
	}
	if *reset {
		if err := devsim.Reset(ctx, pool); err != nil {
			return err
		}
	} else {
		seeded, err := devsim.IsSeeded(ctx, store.New(pool))
		if err != nil {
			return err
		}
		if seeded {
			fmt.Println("Data sudah ada, tidak diubah. Pakai --reset untuk mengisi ulang.")
			return nil
		}
	}
	if err := devsim.Seed(ctx, pool, os.Getenv("SEED_SDM_PASSWORD"), time.Now()); err != nil {
		return err
	}
	fmt.Printf("Seed selesai. Akun SDM: %s (kata sandi = SEED_SDM_PASSWORD di .env)\n", devsim.SeedSDMEmail)
	return nil
}
