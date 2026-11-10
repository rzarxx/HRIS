package attendance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rzarxx/HRIS/backend/internal/face"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

// MaxSelfieBytes adalah batas satu selfie. Aplikasi mengirim foto wajah yang sudah diperkecil,
// jadi foto yang lebih besar menandakan klien yang salah atau upaya membebani server.
const MaxSelfieBytes = 1 << 20

func errTooLarge() error {
	return httpx.NewError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran foto terlalu besar.")
}

// readUpload membaca body multipart (bagian `payload` berisi JSON dan berkas JPEG di field) atau
// JSON biasa. JSON biasa hanya berguna di development, tempat presensi boleh tanpa selfie (D36).
func readUpload(r *http.Request, payload any, field string, maxFiles int) ([][]byte, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "multipart/form-data" {
		return nil, httpx.Decode(r, payload)
	}
	if err := r.ParseMultipartForm(MaxSelfieBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errTooLarge()
		}
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_FAILED", "Body multipart tidak valid.")
	}
	defer r.MultipartForm.RemoveAll()

	parts := r.MultipartForm.Value["payload"]
	if len(parts) != 1 {
		return nil, httpx.Validation(map[string]string{"payload": "Bagian payload wajib dikirim satu kali."})
	}
	if err := httpx.DecodeFrom(strings.NewReader(parts[0]), payload); err != nil {
		return nil, err
	}
	headers := r.MultipartForm.File[field]
	if len(headers) > maxFiles {
		return nil, httpx.Validation(map[string]string{field: fmt.Sprintf("Maksimal %d foto.", maxFiles)})
	}
	files := make([][]byte, 0, len(headers))
	for _, h := range headers {
		if h.Size > MaxSelfieBytes {
			return nil, errTooLarge()
		}
		f, err := h.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, MaxSelfieBytes+1))
		f.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > MaxSelfieBytes {
			return nil, errTooLarge()
		}
		if !face.IsJPEG(data) {
			return nil, httpx.NewError(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Foto harus berformat JPEG.")
		}
		files = append(files, data)
	}
	return files, nil
}

// SelfieStore menyimpan selfie untuk audit (§5.5).
type SelfieStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// DiskStore menyimpan selfie terenkripsi di disk server. Kuncinya sama dengan kunci kolom
// terenkripsi, jadi berkas yang tersalin keluar server tidak bisa dibuka.
type DiskStore struct {
	dir    string
	cipher Cipher
}

func NewDiskStore(dir string, cipher Cipher) (*DiskStore, error) {
	if dir == "" || cipher == nil {
		return nil, errors.New("folder selfie dan cipher wajib diisi")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &DiskStore{dir: dir, cipher: cipher}, nil
}

func (d *DiskStore) path(key string) (string, error) {
	clean := filepath.FromSlash(key)
	if strings.HasPrefix(key, "/") || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("kunci selfie tidak valid: %q", key)
	}
	return filepath.Join(d.dir, clean), nil
}

func (d *DiskStore) Put(_ context.Context, key string, data []byte) error {
	path, err := d.path(key)
	if err != nil {
		return err
	}
	enc, err := d.cipher.Encrypt(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (d *DiskStore) Delete(_ context.Context, key string) error {
	path, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (d *DiskStore) Get(_ context.Context, key string) ([]byte, error) {
	path, err := d.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	enc, err := io.ReadAll(io.LimitReader(f, MaxSelfieBytes+1025))
	if err != nil {
		return nil, err
	}
	if len(enc) > MaxSelfieBytes+1024 {
		return nil, errors.New("berkas selfie terlalu besar")
	}
	return d.cipher.Decrypt(enc)
}
