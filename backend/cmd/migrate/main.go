// Command migrate menjalankan migrasi database memakai role hris_migrate.
// Migrasi hanya maju (MASTER.md Lampiran C), jadi perintah yang tersedia hanya "up" dan "version".
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"strings"

	"github.com/rzarxx/HRIS/backend/internal/platform/config"
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
	dsn := os.Getenv("MIGRATE_DATABASE_URL")
	if dsn == "" {
		return errors.New("MIGRATE_DATABASE_URL wajib diisi")
	}
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	m, err := migrate.New("file://migrations", strings.Replace(dsn, "postgres://", "pgx5://", 1))
	if err != nil {
		return err
	}
	defer m.Close()

	switch command {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
	case "version":
	default:
		return fmt.Errorf("perintah tidak dikenal: %s (pakai up atau version)", command)
	}
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	fmt.Printf("versi migrasi: %d (dirty=%v)\n", version, dirty)
	return nil
}
