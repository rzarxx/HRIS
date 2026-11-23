package filestore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
)

func TestS3StoreEncryptsAndUsesPrivateBucket(t *testing.T) {
	cipher, err := fieldcrypt.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("permintaan S3 tanpa tanda tangan")
		}
		if !strings.HasPrefix(r.URL.Path, "/private/") {
			t.Errorf("bucket atau path salah: %s", r.URL.Path)
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			objects[r.URL.Path] = data
		case http.MethodGet:
			data, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
		default:
			t.Errorf("metode tidak dikenal: %s", r.Method)
		}
	}))
	defer server.Close()
	store, err := NewS3Store(S3Config{Endpoint: server.URL, Bucket: "private", Region: "us-east-1", AccessKey: "test", SecretKey: "test"}, cipher)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "announcements/id/file.pdf"
	plain := []byte("private attachment")
	if err := store.Put(ctx, key, plain); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	raw := bytes.Clone(objects["/private/"+key])
	mu.Unlock()
	if bytes.Contains(raw, plain) {
		t.Fatal("plaintext terkirim ke object storage")
	}
	got, err := store.Get(ctx, key)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Get: %q, %v", got, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete harus idempoten: %v", err)
	}
	for _, bad := range []string{"", "../secret", "/absolute", "a//b", "a\\b"} {
		if err := store.Put(ctx, bad, plain); err == nil {
			t.Errorf("kunci %q diterima", bad)
		}
	}
}
