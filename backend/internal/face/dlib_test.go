//go:build dlib

package face

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Tes ini memakai model dan foto dari github.com/Kagami/go-face-testdata yang diunduh CI ke
// FACE_TESTDATA_DIR. Foto tidak disimpan di repo. Nama berkas menandai orangnya: bona.jpg dan
// bona2.jpg adalah orang yang sama.
func testdata(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("FACE_TESTDATA_DIR")
	if dir == "" {
		t.Skip("FACE_TESTDATA_DIR kosong")
	}
	return dir
}

func newEngine(t *testing.T, dir string) Engine {
	t.Helper()
	e, err := NewDlib(filepath.Join(dir, "models"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Model() != ModelDlib {
		t.Errorf("model = %s", e.Model())
	}
	return e
}

func embedFile(t *testing.T, e Engine, path string) ([]float64, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return e.Embed(context.Background(), data)
}

func TestDlibEngineErrors(t *testing.T) {
	dir := testdata(t)
	e := newEngine(t, dir)
	if _, err := embedFile(t, e, filepath.Join(dir, "group", "pristin.jpg")); !errors.Is(err, ErrManyFaces) {
		t.Errorf("foto grup: %v", err)
	}
	if _, err := e.Embed(context.Background(), []byte("bukan gambar")); !errors.Is(err, ErrBadImage) {
		t.Errorf("bukan JPEG: %v", err)
	}
	if _, err := e.Embed(context.Background(), []byte{0xFF, 0xD8, 0xFF, 0x00}); !errors.Is(err, ErrBadImage) {
		t.Errorf("JPEG rusak: %v", err)
	}
}

type stats struct {
	n              int
	min, max, mean float64
}

func (s *stats) add(d float64) {
	if s.n == 0 || d < s.min {
		s.min = d
	}
	if d > s.max {
		s.max = d
	}
	s.mean += (d - s.mean) / float64(s.n+1)
	s.n++
}

// TestDlibEngine menguji ambang terhadap semua pasangan foto contoh: tidak boleh ada dua orang
// berbeda yang lolos (jarak ≤ MaxDistanceReview). Statistiknya dicatat untuk kalibrasi.
func TestDlibEngine(t *testing.T) {
	dir := testdata(t)
	e := newEngine(t, dir)
	paths, err := filepath.Glob(filepath.Join(dir, "images", "*.jpg"))
	if err != nil || len(paths) < 10 {
		t.Fatalf("foto contoh kurang: %d (%v)", len(paths), err)
	}
	sort.Strings(paths)

	type sample struct {
		name, person string
		vector       []float64
	}
	var samples []sample
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".jpg")
		v, err := embedFile(t, e, p)
		if err != nil {
			t.Logf("lewati %s: %v", name, err)
			continue
		}
		if len(v) != 128 {
			t.Fatalf("%s: dimensi %d", name, len(v))
		}
		samples = append(samples, sample{name: name, person: strings.TrimRight(name, "0123456789"), vector: v})
	}

	var same, other stats
	falseAccept, falseReject, borderline := 0, 0, 0
	for i := range samples {
		for j := i + 1; j < len(samples); j++ {
			a, b := samples[i], samples[j]
			d := Distance(a.vector, b.vector)
			if a.person == b.person {
				same.add(d)
				switch {
				case d > MaxDistanceReview:
					falseReject++
					t.Logf("orang sama ditolak: %s-%s %.3f", a.name, b.name, d)
				case d > MaxDistancePass:
					borderline++
				}
				continue
			}
			other.add(d)
			if d <= MaxDistanceReview {
				falseAccept++
				t.Errorf("orang berbeda lolos: %s-%s %.3f", a.name, b.name, d)
			}
		}
	}
	t.Logf("%d foto terbaca dari %d", len(samples), len(paths))
	t.Logf("orang sama: %d pasang, jarak min %.3f, rata-rata %.3f, maks %.3f", same.n, same.min, same.mean, same.max)
	t.Logf("orang berbeda: %d pasang, jarak min %.3f, rata-rata %.3f, maks %.3f", other.n, other.min, other.mean, other.max)
	t.Logf("ambang lolos %.2f, borderline %.2f: salah terima %d, borderline orang sama %d, salah tolak orang sama %d",
		MaxDistancePass, MaxDistanceReview, falseAccept, borderline, falseReject)
	if same.n == 0 || other.n == 0 || math.IsNaN(same.mean) || same.mean >= other.mean {
		t.Errorf("orang sama harus lebih dekat daripada orang berbeda")
	}
}
