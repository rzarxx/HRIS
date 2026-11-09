package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
)

type retentionStore struct {
	deleted []string
	fail    bool
}

func (s *retentionStore) Put(context.Context, string, []byte) error   { return nil }
func (s *retentionStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (s *retentionStore) Delete(_ context.Context, key string) error {
	if s.fail {
		return errors.New("storage unavailable")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func TestSelfieRetention(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	ctx := context.Background()
	now := time.Now().UTC()
	oldHash, newHash := strings.Repeat("a", 64), strings.Repeat("b", 64)
	exec(t, `INSERT INTO face_captures (employee_id,purpose,sha256,storage_key,created_at) VALUES ($1,'clock_in',$2,'old', $3), ($1,'clock_out',$4,'new', $5)`, f.account.employeeID, oldHash, now.Add(-91*24*time.Hour), newHash, now.Add(-89*24*time.Hour))
	storage := &retentionStore{fail: true}
	if _, err := attendance.PurgeExpiredSelfies(ctx, appPool, storage, now); err == nil {
		t.Fatal("kegagalan penyimpanan harus dilaporkan")
	}
	var marked bool
	if err := ownerPool.QueryRow(ctx, `SELECT selfie_deleted_at IS NOT NULL FROM face_captures WHERE sha256=$1`, oldHash).Scan(&marked); err != nil || marked {
		t.Fatalf("penghapusan gagal tidak boleh ditandai: %v, %v", marked, err)
	}
	storage.fail = false
	count, err := attendance.PurgeExpiredSelfies(ctx, appPool, storage, now)
	if err != nil || count != 1 || len(storage.deleted) != 1 || storage.deleted[0] != "old" {
		t.Fatalf("hasil retensi: %d %v %v", count, storage.deleted, err)
	}
	if err := ownerPool.QueryRow(ctx, `SELECT selfie_deleted_at IS NOT NULL FROM face_captures WHERE sha256=$1`, oldHash).Scan(&marked); err != nil || !marked {
		t.Fatalf("hash dan penanda harus tetap ada: %v, %v", marked, err)
	}
	if _, err := ownerPool.Exec(ctx, `INSERT INTO face_captures (employee_id,purpose,sha256,storage_key) VALUES ($1,'clock_in',$2,'replay')`, f.account.employeeID, oldHash); err == nil {
		t.Fatal("hash lama tetap harus menolak replay")
	}
	count, err = attendance.PurgeExpiredSelfies(ctx, appPool, storage, now)
	if err != nil || count != 0 {
		t.Fatalf("ulangan harus aman: %d %v", count, err)
	}
}
