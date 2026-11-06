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
		distance float64
		want     string
	}{
		{0, "1"}, {0.35, "0.65"}, {0.5, "0.5"}, {0.6, "0.4"}, {1.3, "0"},
	} {
		if got := scoreOf(c.distance).Decimal.String(); got != c.want {
			t.Errorf("jarak %v: skor %s, harus %s", c.distance, got, c.want)
		}
	}
	// Ambang skor adalah padanan ambang jarak.
	if FaceThresholdPass != 1-face.MaxDistancePass || FaceThresholdReview != 1-face.MaxDistanceReview || FaceThresholdPass <= FaceThresholdReview {
		t.Error("ambang skor tidak sesuai jarak")
	}
}

func TestCheckAgreement(t *testing.T) {
	near := [][]float64{{0, 0}, {0.3, 0}, {0, 0.3}}
	if !checkAgreement(near) {
		t.Error("sampel dekat harus lolos")
	}
	if checkAgreement(append(near, []float64{0.6, 0})) {
		t.Error("sampel jauh harus ditolak")
	}
}

func TestEmbeddingRoundTrip(t *testing.T) {
	v := []float64{0.125, -0.5, 0.25, 1}
	got, err := decodeEmbedding(encodeEmbedding(v))
	if err != nil {
		t.Fatal(err)
	}
	if face.Distance(got, v) != 0 {
		t.Errorf("hasil decode berbeda: %v", got)
	}
	if _, err := decodeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Error("data rusak harus ditolak")
	}
}

func TestFaceProblem(t *testing.T) {
	for _, err := range []error{face.ErrNoFace, face.ErrManyFaces, face.ErrBadImage} {
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
