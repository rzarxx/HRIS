//go:build onnx

package face

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Tes ini butuh ONNX Runtime, model, dan foto contoh go-face-testdata yang diunduh CI:
// FACE_TESTDATA_DIR berisi models/, images/, dan group/; ONNXRUNTIME_LIB menunjuk pustakanya; dan
// FACE_REFERENCE berisi embedding acuan OpenCV dari backend/tools/face-bench/reference.py.
// Nama berkas menandai orangnya: bona.jpg dan bona2.jpg adalah orang yang sama.
func newTestEngine(t *testing.T) (*sfaceEngine, string) {
	t.Helper()
	dir, lib := os.Getenv("FACE_TESTDATA_DIR"), os.Getenv("ONNXRUNTIME_LIB")
	if dir == "" || lib == "" {
		t.Skip("FACE_TESTDATA_DIR atau ONNXRUNTIME_LIB kosong")
	}
	e, err := NewSFace(filepath.Join(dir, "models"), lib)
	if err != nil {
		t.Fatal(err)
	}
	if e.Model() != ModelSFace {
		t.Errorf("model = %s", e.Model())
	}
	return e.(*sfaceEngine), dir
}

func images(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "images", "*.jpg"))
	if err != nil || len(paths) < 10 {
		t.Fatalf("foto contoh kurang: %d (%v)", len(paths), err)
	}
	sort.Strings(paths)
	return paths
}

// Embedding Go harus setara dengan OpenCV, karena ambang dipilih dari uji banding OpenCV.
func TestSFaceMatchesOpenCV(t *testing.T) {
	e, dir := newTestEngine(t)
	raw, err := os.ReadFile(os.Getenv("FACE_REFERENCE"))
	if err != nil {
		t.Fatalf("embedding acuan: %v", err)
	}
	var reference map[string][]float64
	if err := json.Unmarshal(raw, &reference); err != nil {
		t.Fatal(err)
	}
	compared, worst := 0, 1.0
	for _, path := range images(t, dir) {
		name := filepath.Base(path)
		want, ok := reference[name]
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := decodeJPEG(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		faces, err := e.Detect(img)
		if err != nil {
			t.Fatal(err)
		}
		if len(faces) == 0 {
			if ok {
				t.Errorf("%s: OpenCV menemukan wajah, Go tidak", name)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: Go menemukan wajah, OpenCV tidak", name)
			continue
		}
		largest := faces[0]
		for _, f := range faces[1:] {
			if f.W*f.H > largest.W*largest.H {
				largest = f
			}
		}
		got, err := e.embedFace(img, largest)
		if err != nil {
			t.Fatal(err)
		}
		sim := Cosine(got, want)
		worst = min(worst, sim)
		compared++
		// Selisih kecil wajar: Go dan libjpeg (OpenCV) mendekode JPEG sedikit berbeda, dan OpenCV memakai
		// aritmetika titik tetap saat resize dan warpAffine. Di bawah 0.97 berarti langkahnya berbeda.
		if sim < 0.97 {
			t.Errorf("%s: kemiripan dengan OpenCV %.4f", name, sim)
		}
	}
	t.Logf("%d foto dibandingkan, kemiripan terendah dengan OpenCV %.4f", compared, worst)
	if compared < 10 {
		t.Errorf("hanya %d foto yang bisa dibandingkan", compared)
	}
}

func TestSFaceEngineErrors(t *testing.T) {
	e, dir := newTestEngine(t)
	group, err := os.ReadFile(filepath.Join(dir, "group", "pristin.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Embed(context.Background(), group); !errors.Is(err, ErrManyFaces) {
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

func (s *stats) add(v float64) {
	if s.n == 0 || v < s.min {
		s.min = v
	}
	if s.n == 0 || v > s.max {
		s.max = v
	}
	s.mean += (v - s.mean) / float64(s.n+1)
	s.n++
}

// TestSFaceEngine menguji ambang terhadap semua pasangan foto contoh lewat jalur produksi (Embed):
// tidak boleh ada dua orang berbeda yang lolos (kemiripan ≥ MinSimilarityReview).
func TestSFaceEngine(t *testing.T) {
	e, dir := newTestEngine(t)
	type sample struct {
		name, person string
		vector       []float64
	}
	var samples []sample
	var turns stats
	for _, path := range images(t, dir) {
		name := strings.TrimSuffix(filepath.Base(path), ".jpg")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		v, err := e.Embed(context.Background(), data)
		if err != nil {
			t.Logf("lewati %s: %v", name, err)
			continue
		}
		if len(v) != 128 {
			t.Fatalf("%s: dimensi %d", name, len(v))
		}
		// Rasio toleh foto contoh dicatat sebagai data awal kalibrasi ambang liveness (D50).
		if a, err := e.Analyze(context.Background(), data); err == nil {
			turns.add(math.Abs(a.Turn))
		}
		samples = append(samples, sample{name: name, person: strings.TrimRight(name, "0123456789"), vector: v})
	}

	var same, other stats
	falseAccept, borderline, rejected := 0, 0, 0
	for i := range samples {
		for j := i + 1; j < len(samples); j++ {
			a, b := samples[i], samples[j]
			s := Cosine(a.vector, b.vector)
			if a.person == b.person {
				same.add(s)
				switch {
				case s < MinSimilarityReview:
					rejected++
				case s < MinSimilarityPass:
					borderline++
				}
				continue
			}
			other.add(s)
			if s >= MinSimilarityReview {
				falseAccept++
				t.Errorf("orang berbeda lolos: %s-%s %.3f", a.name, b.name, s)
			}
		}
	}
	t.Logf("%d foto terbaca", len(samples))
	t.Logf("rasio toleh foto contoh: min %.3f, rata-rata %.3f, maks %.3f (batas lurus %.2f)", turns.min, turns.mean, turns.max, StraightTurnMax)
	t.Logf("orang sama: %d pasang, kemiripan min %.3f, rata-rata %.3f, maks %.3f", same.n, same.min, same.mean, same.max)
	t.Logf("orang berbeda: %d pasang, kemiripan min %.3f, rata-rata %.3f, maks %.3f", other.n, other.min, other.mean, other.max)
	t.Logf("ambang lolos %.2f, borderline %.2f: salah terima %d, borderline orang sama %d, ditolak orang sama %d dari %d",
		MinSimilarityPass, MinSimilarityReview, falseAccept, borderline, rejected, same.n)
	if same.n == 0 || other.n == 0 || same.mean <= other.mean {
		t.Error("orang sama harus lebih mirip daripada orang berbeda")
	}
}
