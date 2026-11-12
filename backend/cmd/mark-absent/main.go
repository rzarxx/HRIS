// Command mark-absent menjalankan job mark_absent (MASTER.md §11.3): mencatat alpha untuk hari kerja
// tanpa presensi, cuti, atau libur. Jalankan harian setelah cutoff 04.00 WIB oleh scheduler host.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
)

func main() {
	days := flag.Int("days", 7, fmt.Sprintf("jumlah hari kerja tertutup terakhir yang diperiksa (1-%d), agar hari yang terlewat saat server mati ikut tercatat", attendance.MaxAbsentLookbackDays))
	flag.Parse()
	if err := run(*days); err != nil {
		fmt.Fprintln(os.Stderr, "gagal:", err)
		os.Exit(1)
	}
}

func run(days int) error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	now := time.Now()
	from, to, err := attendance.AbsentRange(now, days)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	n, err := attendance.MarkAbsent(ctx, pool, now, days)
	if err != nil {
		return err
	}
	fmt.Printf("alpha dicatat: %d (%s s.d. %s)\n", n, from.Format("2006-01-02"), to.Format("2006-01-02"))
	return nil
}
