package filestore

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

// ManagedStore switches new writes to S3 while retaining the local store as a
// read fallback until the operator has verified and retired the disk copy.
type ManagedStore struct {
	mu      sync.RWMutex
	primary Store
	disk    Store
	s3      *S3Store
	refresh func(context.Context) error
}

func NewManagedStore(primary, disk Store, s3 *S3Store) *ManagedStore {
	return &ManagedStore{primary: primary, disk: disk, s3: s3}
}

func (m *ManagedStore) SetRefresh(fn func(context.Context) error) { m.refresh = fn }

func (m *ManagedStore) Activate(s *S3Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.primary, m.s3 = s, s
}

func (m *ManagedStore) CurrentS3() *S3Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.s3
}

func (m *ManagedStore) refreshNow(ctx context.Context) error {
	if m.refresh != nil {
		return m.refresh(ctx)
	}
	return nil
}

func (m *ManagedStore) Put(ctx context.Context, key string, data []byte) error {
	if err := m.refreshNow(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := m.primary
	if p == nil {
		return unavailable()
	}
	return p.Put(ctx, key, data)
}

func (m *ManagedStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := m.refreshNow(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, disk, s := m.primary, m.disk, m.s3
	if p == nil {
		return nil, unavailable()
	}
	data, err := p.Get(ctx, key)
	if err == nil || s == nil || disk == nil || !IsMissing(err) {
		return data, err
	}
	return disk.Get(ctx, key)
}

func (m *ManagedStore) Delete(ctx context.Context, key string) error {
	if err := m.refreshNow(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, disk, s := m.primary, m.disk, m.s3
	if p == nil {
		return unavailable()
	}
	if err := p.Delete(ctx, key); err != nil {
		return err
	}
	if s != nil && disk != nil {
		return disk.Delete(ctx, key)
	}
	return nil
}

func unavailable() error {
	return httpx.NewError(http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Penyimpanan berkas privat belum tersedia. Hubungi pengelola server.")
}

func (m *ManagedStore) Check(ctx context.Context) error {
	if err := m.refreshNow(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.s3
	if s != nil {
		return s.Check(ctx)
	}
	return nil
}

// IsMissing is deliberately narrow: a network or auth failure must not be
// mistaken for an absent object during migration or normal reads.
func IsMissing(err error) bool {
	var noKey *types.NoSuchKey
	var notFound *types.NotFound
	var api smithy.APIError
	return errors.As(err, &noKey) || errors.As(err, &notFound) ||
		(errors.As(err, &api) && (api.ErrorCode() == "NoSuchKey" || api.ErrorCode() == "NotFound" || api.ErrorCode() == "404"))
}
