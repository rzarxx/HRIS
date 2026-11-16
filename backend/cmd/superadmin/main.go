// Command superadmin memberi atau mencabut role super admin dari server (D25).
// Dipakai: hris-superadmin grant --email orang@contoh.id
//
//	hris-superadmin revoke --email orang@contoh.id
//	hris-superadmin reset --email orang@contoh.id
//
// --email juga menerima NIPY atau nomor WA (D53).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

// openTerminal membuka terminal interaktif untuk menampilkan kata sandi awal. Kata sandi tidak
// pernah ditulis ke stdout, agar tidak ikut tersimpan saat keluaran perintah dialihkan ke log.
func openTerminal() (io.WriteCloser, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return nil, errors.New("kata sandi awal hanya ditampilkan di terminal interaktif; jalankan perintah ini langsung di terminal server")
	}
	return tty, nil
}

func showInitialPassword(tty io.Writer, credential auth.InitialCredential) {
	fmt.Fprintf(tty, "Kata sandi awal: %s\nBerlaku sampai %s dan wajib diganti saat login pertama.\n", credential.InitialPassword, credential.ExpiresAt)
}

func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "grant" && os.Args[1] != "revoke" && os.Args[1] != "reset") {
		return errors.New("pemakaian: hris-superadmin grant|revoke|reset --email <email, NIPY, atau nomor WA>")
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	email := flags.String("email", "", "email, NIPY, atau nomor WA akun Pejuang")
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
	// grant dan reset bisa menerbitkan kata sandi awal; pastikan terminal tersedia sebelum
	// mengubah apa pun, supaya kata sandi tidak hilang.
	var tty io.WriteCloser
	if os.Args[1] != "revoke" {
		var err error
		if tty, err = openTerminal(); err != nil {
			return err
		}
		defer tty.Close()
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch os.Args[1] {
	case "revoke":
		if err := auth.RevokeSuperadmin(ctx, pool, *email); err != nil {
			return err
		}
		fmt.Println("Role super admin dicabut. Akun ini kembali menjadi Pejuang.")
	case "reset":
		credential, err := auth.ResetSuperadmin(ctx, pool, *email, time.Now())
		if err != nil {
			return err
		}
		fmt.Println("Kata sandi lama tidak berlaku lagi.")
		showInitialPassword(tty, credential)
	default:
		credential, err := auth.GrantSuperadmin(ctx, pool, *email, time.Now())
		if err != nil {
			return err
		}
		fmt.Println("Akun ini sekarang super admin. Sesi lamanya dicabut.")
		if credential != nil {
			fmt.Println("Akun diaktifkan.")
			showInitialPassword(tty, *credential)
		}
	}
	return nil
}
