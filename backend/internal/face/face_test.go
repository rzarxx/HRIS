package face

import (
	"math"
	"testing"
)

func TestDistanceAndMean(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{3, 4, 0}
	if d := Distance(a, b); d != 5 {
		t.Errorf("jarak = %v", d)
	}
	if !math.IsInf(Distance(a, b[:2]), 1) || !math.IsInf(Distance(nil, nil), 1) {
		t.Error("dimensi berbeda harus tak hingga")
	}
	m := Mean([][]float64{{1, 2}, {3, 6}})
	if m[0] != 2 || m[1] != 4 {
		t.Errorf("rata-rata = %v", m)
	}
	if Mean(nil) != nil {
		t.Error("rata-rata kosong")
	}
}

func TestIsJPEG(t *testing.T) {
	if !IsJPEG([]byte{0xFF, 0xD8, 0xFF, 0xE0}) || IsJPEG([]byte("\x89PNG")) || IsJPEG(nil) {
		t.Error("deteksi JPEG")
	}
}
