package attendance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// PurgeExpiredSelfies menghapus berkas lebih lama dari 90 hari tanpa menghapus hash anti-replay.
// Satu transaksi per berkas membuat pekerjaan dapat dilanjutkan setelah kegagalan; Delete harus
// menerima berkas yang sudah tidak ada karena proses mungkin terhenti sebelum commit.
func PurgeExpiredSelfies(ctx context.Context, pool *pgxpool.Pool, selfies SelfieStore, now time.Time) (int, error) {
	if pool == nil || selfies == nil {
		return 0, errors.New("database dan penyimpanan selfie wajib diisi")
	}
	cutoff := pgtype.Timestamptz{Time: now.UTC().Add(-90 * 24 * time.Hour), Valid: true}
	count := 0
	for {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return count, err
		}
		q := store.New(tx)
		capture, err := q.LockExpiredFaceCapture(ctx, cutoff)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return count, nil
		}
		if err == nil {
			err = selfies.Delete(ctx, capture.StorageKey)
		}
		if err == nil {
			err = q.MarkFaceCaptureDeleted(ctx, capture.ID)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return count, fmt.Errorf("hapus selfie kedaluwarsa: %w", err)
		}
		if err = tx.Commit(ctx); err != nil {
			return count, err
		}
		count++
	}
}
