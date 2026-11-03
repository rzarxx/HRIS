// Command superadmin memberi atau mencabut role super admin dari server (D25).
// Dipakai: hris-superadmin grant --email orang@contoh.id
//
//	hris-superadmin revoke --email orang@contoh.id
//	hris-superadmin reset --email orang@contoh.id
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gagal:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "grant" && os.Args[1] != "revoke" && os.Args[1] != "reset") {
		return errors.New("pemakaian: hris-superadmin grant|revoke|reset --email <email>")
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	email := flags.String("email", "", "email akun Pejuang")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email wajib diisi")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	if os.Args[1] == "revoke" {
		if err := auth.RevokeSuperadmin(ctx, pool, *email); err != nil {
			return err
		}
		fmt.Println("Role super admin dicabut. Akun ini kembali menjadi Pejuang.")
		return nil
	}
	if os.Args[1] == "reset" {
		invite, err := auth.ResetSuperadmin(ctx, pool, *email, time.Now())
		if err != nil {
			return err
		}
		fmt.Printf("Kata sandi lama tidak berlaku lagi. Buka alamat web admin + /aktivasi/?token=%s\nBerlaku sampai %s.\n", invite.Token, invite.ExpiresAt)
		return nil
	}
	activation, err := auth.GrantSuperadmin(ctx, pool, *email, time.Now())
	if err != nil {
		return err
	}
	fmt.Println("Akun ini sekarang super admin. Sesi lamanya dicabut.")
	if activation != nil {
		fmt.Printf("Akun belum aktif. Buka alamat web admin + /aktivasi/?token=%s\nBerlaku sampai %s.\n", activation.Token, activation.ExpiresAt)
	}
	return nil
}
