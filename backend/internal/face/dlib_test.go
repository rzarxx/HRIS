//go:build dlib

package face

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Tes ini memakai model dan foto dari github.com/Kagami/go-face-testdata yang diunduh CI ke
// FACE_TESTDATA_DIR. Foto tidak disimpan di repo.
func testdata(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("FACE_TESTDATA_DIR")
	if dir == "" {
		t.Skip("FACE_TESTDATA_DIR kosong")
	}
	return dir
}

func embedFile(t *testing.T, e Engine, path string) ([]float64, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return e.Embed(context.Background(), data)
}

func TestDlibEngine(t *testing.T) {
	dir := testdata(t)
	e, err := NewDlib(filepath.Join(dir, "models"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Model() != ModelDlib {
		t.Errorf("model = %s", e.Model())
	}
	img := func(name string) string { return filepath.Join(dir, "images", name) }

	bona, err := embedFile(t, e, img("bona.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bona) != 128 {
		t.Fatalf("dimensi = %d", len(bona))
	}
	same, err := embedFile(t, e, img("bona2.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := embedFile(t, e, img("jimin.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if d := Distance(bona, same); d >= 0.6 {
		t.Errorf("orang sama berjarak %v", d)
	}
	if d := Distance(bona, other); d < 0.6 {
		t.Errorf("orang berbeda berjarak %v", d)
	}
	t.Logf("jarak sama %.3f, berbeda %.3f", Distance(bona, same), Distance(bona, other))

	if _, err := embedFile(t, e, img("pristin.jpg")); !errors.Is(err, ErrManyFaces) {
		t.Errorf("foto grup: %v", err)
	}
	if _, err := e.Embed(context.Background(), []byte("bukan gambar")); !errors.Is(err, ErrBadImage) {
		t.Errorf("bukan JPEG: %v", err)
	}
	if _, err := e.Embed(context.Background(), []byte{0xFF, 0xD8, 0xFF, 0x00}); !errors.Is(err, ErrBadImage) {
		t.Errorf("JPEG rusak: %v", err)
	}
}
