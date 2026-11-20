package storageadmin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/platform/filestore"
)

type memoryDisk struct{ data map[string][]byte }

func (d *memoryDisk) Put(_ context.Context, key string, data []byte) error {
	d.data[key] = bytes.Clone(data)
	return nil
}
func (d *memoryDisk) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := d.data[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return bytes.Clone(v), nil
}
func (d *memoryDisk) Delete(_ context.Context, key string) error { delete(d.data, key); return nil }

func TestCopyAndVerifyAndManagedFallback(t *testing.T) {
	cipher, err := fieldcrypt.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	objects := map[string][]byte{}
	failRead := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/private/") {
			t.Errorf("path: %s", r.URL.Path)
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if failRead {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			v, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(v)
		case http.MethodPut:
			v, e := io.ReadAll(r.Body)
			if e != nil {
				t.Error(e)
			}
			objects[r.URL.Path] = v
		case http.MethodDelete:
			delete(objects, r.URL.Path)
		default:
			t.Errorf("method: %s", r.Method)
		}
	}))
	defer server.Close()
	s3, err := filestore.NewS3Store(filestore.S3Config{Endpoint: server.URL, Bucket: "private", Region: "us-east-1", AccessKey: "key", SecretKey: "secret"}, cipher)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	disk := &memoryDisk{data: map[string][]byte{"employees/photo.jpg": []byte("old")}}
	managed := filestore.NewManagedStore(disk, disk, nil)
	managed.Activate(s3)
	if got, e := managed.Get(ctx, "employees/photo.jpg"); e != nil || string(got) != "old" {
		t.Fatalf("fallback: %q %v", got, e)
	}
	mu.Lock()
	failRead = true
	mu.Unlock()
	if _, e := managed.Get(ctx, "employees/photo.jpg"); e == nil {
		t.Fatal("gangguan S3 tidak boleh dianggap objek hilang")
	}
	mu.Lock()
	failRead = false
	mu.Unlock()
	copied, e := copyAndVerify(ctx, disk, s3, "employees/photo.jpg")
	if e != nil || !copied {
		t.Fatalf("copy: %v %v", copied, e)
	}
	copied, e = copyAndVerify(ctx, disk, s3, "employees/photo.jpg")
	if e != nil || copied {
		t.Fatalf("idempotent: %v %v", copied, e)
	}
	if e := s3.Put(ctx, "employees/photo.jpg", []byte("different")); e != nil {
		t.Fatal(e)
	}
	if _, e := copyAndVerify(ctx, disk, s3, "employees/photo.jpg"); e == nil {
		t.Fatal("konflik isi S3 harus ditolak")
	}
	if got, e := managed.Get(ctx, "employees/photo.jpg"); e != nil || string(got) != "different" {
		t.Fatalf("objek lama tertimpa: %q %v", got, e)
	}
	if e := managed.Delete(ctx, "employees/photo.jpg"); e != nil {
		t.Fatal(e)
	}
	if len(disk.data) != 0 {
		t.Fatal("salinan disk tidak dihapus saat retensi")
	}
}

func TestValidateStorageEndpoint(t *testing.T) {
	good := Input{Endpoint: "https://s3.example.org/", Bucket: "hris-private", Region: "us-east-1", AccessKey: "key", SecretKey: "secret"}
	if f := validate(&good); len(f) != 0 || good.Endpoint != "https://s3.example.org" {
		t.Fatalf("endpoint valid ditolak: %v", f)
	}
	for _, endpoint := range []string{"http://s3.example.org", "https://user:pass@s3.example.org", "https://s3.example.org/path"} {
		in := good
		in.Endpoint = endpoint
		if f := validate(&in); f["endpoint"] == "" {
			t.Errorf("endpoint tidak aman diterima: %s", endpoint)
		}
	}
}

func TestListDiskKeysUsesCursorOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"a/x", "a-b", "z"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(key)), []byte("ciphertext"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	keys, more, err := nextDiskKeys(root, "", 1)
	if err != nil || !more || !reflect.DeepEqual(keys, []string{"a/x"}) {
		t.Fatalf("page 1: %v %v %v", keys, more, err)
	}
	keys, more, err = nextDiskKeys(root, "a/x", 2)
	if err != nil || more || !reflect.DeepEqual(keys, []string{"a-b", "z"}) {
		t.Fatalf("page 2: %v %v %v", keys, more, err)
	}
	if err := os.Remove(filepath.Join(root, "a", "x")); err != nil {
		t.Fatal(err)
	}
	keys, more, err = nextDiskKeys(root, "a/x", 2)
	if err != nil || more || !reflect.DeepEqual(keys, []string{"a-b", "z"}) {
		t.Fatalf("deleted cursor: %v %v %v", keys, more, err)
	}
}
