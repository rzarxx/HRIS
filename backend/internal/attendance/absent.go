package attendance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// MaxAbsentLookbackDays membatasi berapa hari ke belakang job mark_absent mengejar.
const MaxAbsentLookbackDays = 62

// ClosedWorkDate adalah tanggal kerja terakhir yang sudah tertutup pada waktu now: semua presensi
// masuk untuk tanggal itu sudah lewat batasnya. Presensi masuk tanggal D paling lambat sebelum cutoff
// harian D+1, termasuk shift yang dimulai sebelum cutoff dan yang menerima datang awal (lihat WorkDate).
func ClosedWorkDate(now time.Time) time.Time {
	return pgconv.WorkDate(now.Add(-DayCutoff)).AddDate(0, 0, -1)
}

// AbsentRange adalah rentang tanggal kerja yang diperiksa job mark_absent: days hari yang berakhir di
// ClosedWorkDate(now). Hari yang sudah dicatat tidak diubah, jadi rentang yang tumpang tindih aman.
func AbsentRange(now time.Time, days int) (from, to time.Time, err error) {
	if days < 1 || days > MaxAbsentLookbackDays {
		return time.Time{}, time.Time{}, fmt.Errorf("jumlah hari harus 1 sampai %d", MaxAbsentLookbackDays)
	}
	to = ClosedWorkDate(now)
	return to.AddDate(0, 0, 1-days), to, nil
}

// MarkAbsent menjalankan job mark_absent (MASTER.md §11.3): mencatat alpha untuk hari kerja shift
// yang tidak libur dan tanpa presensi atau cuti, untuk days hari terakhir yang sudah tertutup.
// Mengembalikan jumlah baris alpha yang baru dicatat.
func MarkAbsent(ctx context.Context, pool *pgxpool.Pool, now time.Time, days int) (int64, error) {
	if pool == nil {
		return 0, errors.New("database wajib diisi")
	}
	from, to, err := AbsentRange(now, days)
	if err != nil {
		return 0, err
	}
	n, err := store.New(pool).MarkAbsentBetween(ctx, store.MarkAbsentBetweenParams{
		FromDate: pgconv.Date(from), ToDate: pgconv.Date(to),
	})
	if err != nil {
		return 0, fmt.Errorf("catat alpha: %w", err)
	}
	return n, nil
}
