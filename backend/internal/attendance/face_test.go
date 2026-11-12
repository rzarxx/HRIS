package attendance

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/face"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
)

func TestScoreOf(t *testing.T) {
	for _, c := range []struct {
		similarity float64
		want       string
	}{
		{1, "1"}, {0.65, "0.65"}, {0.5, "0.5"}, {-0.2, "0"}, {1.3, "1"},
	} {
		if got := scoreOf(c.similarity).Decimal.String(); got != c.want {
			t.Errorf("kemiripan %v: skor %s, harus %s", c.similarity, got, c.want)
		}
	}
	if FaceThresholdPass != face.MinSimilarityPass || FaceThresholdReview != face.MinSimilarityReview || FaceThresholdPass <= FaceThresholdReview {
		t.Error("ambang skor tidak sesuai jarak")
	}
}

func TestCheckAgreement(t *testing.T) {
	near := [][]float64{{1, 0.1}, {1, 0.2}, {1, 0.3}}
	if !checkAgreement(near) {
		t.Error("sampel mirip harus lolos")
	}
	if checkAgreement(append(near, []float64{0, 1})) {
		t.Error("sampel berbeda harus ditolak")
	}
}

func TestEmbeddingRoundTrip(t *testing.T) {
	v := []float64{0.125, -0.5, 0.25, 1}
	got, err := decodeEmbedding(encodeEmbedding(v))
	if err != nil {
		t.Fatal(err)
	}
	if face.Cosine(got, v) < 0.999999 {
		t.Errorf("hasil decode berbeda: %v", got)
	}
	if _, err := decodeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Error("data rusak harus ditolak")
	}
}

func TestFaceProblem(t *testing.T) {
	for _, err := range []error{face.ErrNoFace, face.ErrManyFaces, face.ErrFaceNotCentered, face.ErrBadImage} {
		if msg, ok := faceProblem(err); !ok || msg == "" {
			t.Errorf("%v harus punya pesan", err)
		}
	}
	if _, ok := faceProblem(errors.New("lain")); ok {
		t.Error("error lain bukan masalah foto")
	}
}

func TestFaceBorderlineScore(t *testing.T) {
	if RiskScore([]string{flagFaceBorderline}) != 20 {
		t.Error("face_borderline bernilai 20 poin (§5.4)")
	}
}

func TestLivenessRetrySignal(t *testing.T) {
	if LivenessRetried(1) || !LivenessRetried(2) || !LivenessRetried(5) {
		t.Error("liveness_retry menyala mulai dua upaya gagal (§5.4)")
	}
	if RiskScore([]string{flagLivenessRetry}) != 25 {
		t.Error("liveness_retry bernilai 25 poin (§5.4)")
	}
}

func TestLivenessActionsAreProvable(t *testing.T) {
	for _, a := range LivenessActions {
		if a != "turn_left" && a != "turn_right" {
			t.Errorf("%q tidak bisa dibuktikan server dari foto gerakan (D50)", a)
		}
	}
}

func TestProvesTurn(t *testing.T) {
	person := []float64{1, 0, 0}
	same := []float64{0.9, 0.43, 0}
	other := []float64{0, 1, 0}
	straight := face.Analysis{Vector: person, Turn: 0.02}
	cases := []struct {
		name     string
		straight face.Analysis
		gesture  face.Analysis
		want     bool
	}{
		{"menoleh ke kiri", straight, face.Analysis{Vector: same, Turn: 0.28}, true},
		{"menoleh ke kanan", straight, face.Analysis{Vector: same, Turn: -0.28}, true},
		{"foto gerakan lurus", straight, face.Analysis{Vector: same, Turn: 0.05}, false},
		{"tolehan terlalu kecil", straight, face.Analysis{Vector: same, Turn: 0.15}, false},
		{"selisih terlalu kecil", face.Analysis{Vector: person, Turn: 0.19}, face.Analysis{Vector: same, Turn: 0.25}, false},
		{"foto utama menoleh", face.Analysis{Vector: person, Turn: 0.3}, face.Analysis{Vector: same, Turn: -0.3}, false},
		{"orang lain", straight, face.Analysis{Vector: other, Turn: 0.3}, false},
	}
	for _, c := range cases {
		if got := provesTurn(c.straight, c.gesture); got != c.want {
			t.Errorf("%s: %v, ingin %v", c.name, got, c.want)
		}
	}
}

func TestDiskStoreEncrypts(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := fieldcrypt.New(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store, err := NewDiskStore(dir, cipher)
	if err != nil {
		t.Fatal(err)
	}
	photo := []byte{0xFF, 0xD8, 0xFF, 'f', 'o', 't', 'o'}
	if err := store.Put(context.Background(), "clock_in/2026/11/a.jpg.enc", photo); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "clock_in", "2026", "11", "a.jpg.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, photo) {
		t.Error("selfie tersimpan tanpa enkripsi")
	}
	plain, err := cipher.Decrypt(raw)
	if err != nil || !bytes.Equal(plain, photo) {
		t.Errorf("dekripsi: %v", err)
	}
	loaded, err := store.Get(context.Background(), "clock_in/2026/11/a.jpg.enc")
	if err != nil || !bytes.Equal(loaded, photo) {
		t.Errorf("baca selfie: %v", err)
	}
	for _, bad := range []string{"../luar.jpg", "/abs.jpg", ""} {
		if err := store.Put(context.Background(), bad, photo); err == nil {
			t.Errorf("kunci %q harus ditolak", bad)
		}
	}
	if err := store.Delete(context.Background(), "clock_in/2026/11/a.jpg.enc"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "clock_in/2026/11/a.jpg.enc"); err != nil {
		t.Errorf("hapus berkas yang tidak ada: %v", err)
	}
}
