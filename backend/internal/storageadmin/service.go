// Package storageadmin manages the S3 destination and a resumable disk migration.
package storageadmin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/platform/filestore"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	settingKey   = "storage_s3"
	migrationKey = "storage_s3_migration"
)

type stored struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretEnc string `json:"secret_enc"`
}

type Input struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

type Progress struct {
	Cursor    string  `json:"cursor"`
	Scanned   int     `json:"scanned"`
	Copied    int     `json:"copied"`
	Done      bool    `json:"done"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

type View struct {
	Configured bool     `json:"configured"`
	Active     bool     `json:"active"`
	Endpoint   string   `json:"endpoint"`
	Bucket     string   `json:"bucket"`
	Region     string   `json:"region"`
	AccessKey  string   `json:"access_key"`
	SecretSet  bool     `json:"secret_set"`
	Migration  Progress `json:"migration"`
}

type Service struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	cipher   *fieldcrypt.Cipher
	files    *filestore.ManagedStore
	disk     filestore.Store
	diskRoot string
	mu       sync.Mutex
	loaded   string
	identity string
	initial  filestore.S3Config
}

func NewService(pool *pgxpool.Pool, cipher *fieldcrypt.Cipher, files *filestore.ManagedStore, disk filestore.Store, diskRoot string, initial filestore.S3Config) *Service {
	s := &Service{pool: pool, q: store.New(pool), cipher: cipher, files: files, disk: disk, diskRoot: diskRoot, initial: initial}
	if initial.Endpoint != "" {
		s.identity = initial.Endpoint + "\x00" + initial.Bucket
	}
	return s
}

func (s *Service) load(ctx context.Context) (stored, bool, error) {
	row, err := s.q.GetAppSetting(ctx, settingKey)
	if pgconv.IsNoRows(err) {
		return stored{}, false, nil
	}
	if err != nil {
		return stored{}, false, err
	}
	var cfg stored
	if err := json.Unmarshal([]byte(row.Value), &cfg); err != nil {
		return stored{}, false, err
	}
	return cfg, true, nil
}

func (s *Service) build(st stored) (*filestore.S3Store, error) {
	raw, err := base64.StdEncoding.DecodeString(st.SecretEnc)
	if err != nil {
		return nil, err
	}
	plain, err := s.cipher.Decrypt(raw)
	if err != nil {
		return nil, err
	}
	return filestore.NewS3Store(filestore.S3Config{Endpoint: st.Endpoint, Bucket: st.Bucket, Region: st.Region, AccessKey: st.AccessKey, SecretKey: string(plain)}, s.cipher)
}

// Refresh runs before every file operation so other API instances use a saved
// destination before accepting another write. A DB error fails closed.
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, err := s.q.GetAppSetting(ctx, settingKey)
	if pgconv.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Value == s.loaded {
		return nil
	}
	var st stored
	if err := json.Unmarshal([]byte(row.Value), &st); err != nil {
		return err
	}
	client, err := s.build(st)
	if err != nil {
		return err
	}
	if s.identity != "" && s.identity != st.Endpoint+"\x00"+st.Bucket {
		return errors.New("tujuan S3 aktif berubah tanpa migrasi")
	}
	s.files.Activate(client)
	s.identity, s.loaded = st.Endpoint+"\x00"+st.Bucket, row.Value
	return nil
}

func (s *Service) progress(ctx context.Context) (Progress, error) {
	row, err := s.q.GetAppSetting(ctx, migrationKey)
	if pgconv.IsNoRows(err) {
		return Progress{}, nil
	}
	if err != nil {
		return Progress{}, err
	}
	var p Progress
	if err := json.Unmarshal([]byte(row.Value), &p); err != nil {
		return Progress{}, err
	}
	p.UpdatedAt = pgconv.TimeString(row.UpdatedAt)
	return p, nil
}

func (s *Service) saveProgress(ctx context.Context, p Progress, entry audit.Entry) error {
	value, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: migrationKey, Value: string(value), UpdatedBy: entry.UserID})
}

func (s *Service) Settings(ctx context.Context) (View, error) {
	if err := s.Refresh(ctx); err != nil {
		return View{}, err
	}
	st, ok, err := s.load(ctx)
	if err != nil {
		return View{}, err
	}
	p, err := s.progress(ctx)
	if err != nil {
		return View{}, err
	}
	if !ok && s.initial.Endpoint != "" {
		st = stored{Endpoint: s.initial.Endpoint, Bucket: s.initial.Bucket, Region: s.initial.Region, AccessKey: s.initial.AccessKey}
	}
	return View{Configured: ok || s.initial.Endpoint != "", Active: s.files.CurrentS3() != nil, Endpoint: st.Endpoint, Bucket: st.Bucket, Region: st.Region, AccessKey: st.AccessKey, SecretSet: st.SecretEnc != "" || !ok && s.initial.SecretKey != "", Migration: p}, nil
}

func validate(in *Input) map[string]string {
	in.Endpoint = strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	in.Bucket = strings.TrimSpace(in.Bucket)
	in.Region = strings.TrimSpace(in.Region)
	in.AccessKey = strings.TrimSpace(in.AccessKey)
	f := map[string]string{}
	u, err := url.Parse(in.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback()))) || len(in.Endpoint) > 512 {
		f["endpoint"] = "Gunakan URL HTTPS tanpa path, atau HTTP loopback untuk pengujian lokal."
	}
	if len(in.Bucket) < 3 || len(in.Bucket) > 63 || strings.ContainsAny(in.Bucket, " /\\\r\n") {
		f["bucket"] = "Nama bucket harus 3–63 karakter tanpa spasi."
	}
	if in.Region == "" || len(in.Region) > 64 || strings.ContainsAny(in.Region, "\r\n") {
		f["region"] = "Region wajib diisi (contoh: us-east-1)."
	}
	if in.AccessKey == "" || len(in.AccessKey) > 256 || strings.ContainsAny(in.AccessKey, "\r\n") {
		f["access_key"] = "Access key wajib diisi."
	}
	if len(in.SecretKey) > 256 || strings.ContainsAny(in.SecretKey, "\r\n") {
		f["secret_key"] = "Secret key maksimal 256 karakter."
	}
	return f
}

// Save activates S3 only after a complete write/read/delete probe succeeds.
// The disk copy is deliberately retained for fallback and independent backup.
func (s *Service) Save(ctx context.Context, entry audit.Entry, in Input) (View, error) {
	fields := validate(&in)
	if len(fields) > 0 {
		return View{}, httpx.Validation(fields)
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return View{}, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(78342160)").Scan(&locked); err != nil {
		return View{}, err
	}
	if !locked {
		return View{}, httpx.NewError(http.StatusConflict, "STORAGE_SYNC_BUSY", "Pengaturan S3 sedang disimpan oleh proses lain. Coba lagi.")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock(78342160)")
	}()
	// s.mu tidak ditahan selama uji koneksi (bisa 15 detik) karena Refresh memakainya di setiap
	// operasi berkas; advisory lock di atas sudah mencegah dua Save berjalan bersamaan.
	s.mu.Lock()
	current := s.identity
	s.mu.Unlock()
	old, had, err := s.load(ctx)
	if err != nil {
		return View{}, err
	}
	identity := in.Endpoint + "\x00" + in.Bucket
	if current != "" && current != identity || had && (old.Endpoint != in.Endpoint || old.Bucket != in.Bucket) {
		return View{}, errDestinationLocked()
	}
	secretEnc := old.SecretEnc
	secret := in.SecretKey
	if secret == "" && !had && s.initial.SecretKey != "" {
		secret = s.initial.SecretKey
	}
	if secret == "" && had {
		raw, err := base64.StdEncoding.DecodeString(old.SecretEnc)
		if err == nil {
			var plain []byte
			plain, err = s.cipher.Decrypt(raw)
			secret = string(plain)
		}
		if err != nil {
			return View{}, err
		}
	}
	if secret == "" {
		return View{}, httpx.Validation(map[string]string{"secret_key": "Secret key wajib diisi."})
	}
	if !had || in.SecretKey != "" {
		enc, err := s.cipher.Encrypt([]byte(secret))
		if err != nil {
			return View{}, err
		}
		secretEnc = base64.StdEncoding.EncodeToString(enc)
	}
	st := stored{Endpoint: in.Endpoint, Bucket: in.Bucket, Region: in.Region, AccessKey: in.AccessKey, SecretEnc: secretEnc}
	client, err := filestore.NewS3Store(filestore.S3Config{Endpoint: in.Endpoint, Bucket: in.Bucket, Region: in.Region, AccessKey: in.AccessKey, SecretKey: secret}, s.cipher)
	if err != nil {
		return View{}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err = client.Probe(probeCtx)
	cancel()
	if err != nil {
		// Galat SDK tidak memuat secret key; dicatat agar operator bisa mendiagnosis.
		slog.WarnContext(ctx, "uji S3 gagal", "endpoint", in.Endpoint, "bucket", in.Bucket, "error", err)
		return View{}, httpx.NewError(http.StatusUnprocessableEntity, "STORAGE_TEST_FAILED", "Koneksi atau izin baca/tulis/hapus pada bucket S3 gagal. Periksa endpoint, bucket, region, dan kredensial.")
	}
	value, err := json.Marshal(st)
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	// Proses lain bisa saja sudah memuat tujuan berbeda selama uji berlangsung.
	if s.identity != "" && s.identity != identity {
		s.mu.Unlock()
		return View{}, errDestinationLocked()
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: settingKey, Value: string(value), UpdatedBy: entry.UserID}); err != nil {
			return err
		}
		e := entry
		e.Action, e.Entity = audit.SettingsChanged, "app_setting"
		e.Detail = map[string]any{"key": settingKey, "endpoint": in.Endpoint, "bucket": in.Bucket, "credentials_changed": in.SecretKey != ""}
		return audit.Write(ctx, q, e)
	})
	if err == nil {
		s.files.Activate(client)
		s.identity = identity
		s.loaded = string(value)
	}
	s.mu.Unlock()
	if err != nil {
		return View{}, err
	}
	return s.Settings(ctx)
}

func errDestinationLocked() error {
	return httpx.NewError(http.StatusConflict, "STORAGE_DESTINATION_LOCKED", "Tujuan S3 yang aktif tidak dapat diganti selama data tersimpan. Rotasi kredensial pada tujuan yang sama diperbolehkan.")
}

// SyncBatch copies at most twenty encrypted disk objects. Each object is read
// back and compared; an existing conflicting S3 object is never overwritten.
func (s *Service) SyncBatch(ctx context.Context, entry audit.Entry) (View, error) {
	if err := s.Refresh(ctx); err != nil {
		return View{}, err
	}
	client := s.files.CurrentS3()
	if client == nil {
		return View{}, httpx.NewError(http.StatusConflict, "STORAGE_NOT_ACTIVE", "Simpan pengaturan S3 terlebih dahulu.")
	}
	if s.disk == nil || s.diskRoot == "" {
		return View{}, httpx.NewError(http.StatusConflict, "STORAGE_SOURCE_MISSING", "SELFIE_DIR lama belum tersedia pada server API.")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return View{}, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(78342159)").Scan(&locked); err != nil {
		return View{}, err
	}
	if !locked {
		return View{}, httpx.NewError(http.StatusConflict, "STORAGE_SYNC_BUSY", "Sinkronisasi sedang berjalan. Coba beberapa saat lagi.")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock(78342159)")
	}()
	p, err := s.progress(ctx)
	if err != nil {
		return View{}, err
	}
	if p.Done {
		p = Progress{} // manual rerun verifies the entire disk copy again
		if err := s.saveProgress(ctx, p, entry); err != nil {
			return View{}, err
		}
	}
	keys, hasMore, err := nextDiskKeys(s.diskRoot, p.Cursor, 20)
	if err != nil {
		return View{}, err
	}
	for _, key := range keys {
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 4*time.Second {
			hasMore = true
			break
		}
		copied, err := copyAndVerify(ctx, s.disk, client, key)
		if err != nil {
			slog.ErrorContext(ctx, "migrasi berkas S3 gagal", "key", key, "error", err)
			return View{}, httpx.NewError(http.StatusUnprocessableEntity, "STORAGE_SYNC_FAILED", "Sinkronisasi berhenti karena berkas lama gagal dibaca atau salinannya di S3 tidak sesuai. Berkas lama tetap tersimpan.")
		}
		if copied {
			p.Copied++
		}
		p.Scanned++
		p.Cursor = key
		if err := s.saveProgress(ctx, p, entry); err != nil {
			return View{}, err
		}
	}
	p.Done = !hasMore
	if err := s.saveProgress(ctx, p, entry); err != nil {
		return View{}, err
	}
	return s.Settings(ctx)
}

// WalkDir sorts each directory's entries. Replacing '/' with NUL yields the
// same order and lets a cursor skip entire earlier directory subtrees.
func cursorOrder(key string) string { return strings.ReplaceAll(key, "/", "\x00") }

func nextDiskKeys(source, cursor string, limit int) ([]string, bool, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return nil, false, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, false, err
	}
	keys := make([]string, 0, limit)
	cursorRank := cursorOrder(cursor)
	stop := errors.New("page complete")
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if d.IsDir() {
			branch := cursorOrder(key) + "\x00"
			if cursorRank > branch && !strings.HasPrefix(cursorRank, branch) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasSuffix(d.Name(), ".tmp") || cursorOrder(key) <= cursorRank {
			return nil
		}
		if len(keys) == limit {
			return stop
		}
		keys = append(keys, key)
		return nil
	})
	if errors.Is(err, stop) {
		return keys, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return keys, false, nil
}

func copyAndVerify(ctx context.Context, disk filestore.Store, remote *filestore.S3Store, key string) (bool, error) {
	local, err := disk.Get(ctx, key)
	if err != nil {
		return false, err
	}
	got, err := remote.Get(ctx, key)
	copied := false
	if filestore.IsMissing(err) {
		if err := remote.Put(ctx, key, local); err != nil {
			return false, err
		}
		copied = true
		got, err = remote.Get(ctx, key)
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(local, got) {
		return false, errors.New("isi S3 berbeda; objek tidak ditimpa")
	}
	return copied, nil
}
