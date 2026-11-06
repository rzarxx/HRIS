package attendance

import (
	"math"
	"testing"
)

// vec membuat embedding uji: basis yang sama ditambah sedikit derau per indeks.
func vec(dim int, base, noise float64, seed int) []float64 {
	out := make([]float64, dim)
	for i := range out {
		out[i] = base*math.Sin(float64(i+1)) + noise*math.Cos(float64((i+1)*(seed+3)))
	}
	return out
}

func TestCosine(t *testing.T) {
	a := vec(128, 1, 0, 0)
	if got := Cosine(a, a); math.Abs(got-1) > 1e-9 {
		t.Errorf("vektor sama = %v", got)
	}
	neg := make([]float64, len(a))
	for i, x := range a {
		neg[i] = -x
	}
	if got := Cosine(a, neg); math.Abs(got+1) > 1e-9 {
		t.Errorf("vektor berlawanan = %v", got)
	}
	if Cosine(a, a[:64]) != 0 || Cosine(nil, nil) != 0 || Cosine(a, make([]float64, 128)) != 0 {
		t.Error("vektor tidak sebanding harus 0")
	}
	nan := append([]float64{math.NaN()}, a[1:]...)
	if Cosine(a, nan) != 0 {
		t.Error("NaN harus 0")
	}
}

func TestBuildTemplate(t *testing.T) {
	samples := [][]float64{vec(128, 1, 0.05, 1), vec(128, 1, 0.05, 2), vec(128, 1, 0.05, 3)}
	tpl, err := BuildTemplate(samples)
	if err != nil {
		t.Fatal(err)
	}
	var norm float64
	for _, x := range tpl {
		norm += x * x
	}
	if math.Abs(norm-1) > 1e-9 {
		t.Errorf("template tidak satuan: %v", norm)
	}
	for i, s := range samples {
		if c := Cosine(tpl, s); c < 0.95 {
			t.Errorf("sampel %d jauh dari template: %v", i, c)
		}
	}

	for name, bad := range map[string][][]float64{
		"terlalu sedikit": samples[:2],
		"terlalu banyak":  append(samples, samples...),
		"dimensi kecil":   {vec(32, 1, 0, 1), vec(32, 1, 0, 2), vec(32, 1, 0, 3)},
		"dimensi beda":    {vec(128, 1, 0, 1), vec(128, 1, 0, 2), vec(129, 1, 0, 3)},
		"vektor nol":      {vec(128, 1, 0, 1), vec(128, 1, 0, 2), make([]float64, 128)},
		"tidak konsisten": {vec(128, 1, 0, 1), vec(128, 1, 0, 2), vec(128, 0, 1, 9)},
	} {
		if _, err := BuildTemplate(bad); err == nil {
			t.Errorf("%s: harus ditolak", name)
		}
	}
}

func TestEmbeddingRoundTrip(t *testing.T) {
	v := vec(192, 1, 0.1, 4)
	got, err := decodeEmbedding(encodeEmbedding(v))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(v) || Cosine(got, v) < 0.999999 {
		t.Errorf("hasil decode berbeda")
	}
	if _, err := decodeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Error("data rusak harus ditolak")
	}
}

func TestFaceBorderlineScore(t *testing.T) {
	if RiskScore([]string{flagFaceBorderline}) != 20 {
		t.Error("face_borderline bernilai 20 poin (§5.4)")
	}
}
