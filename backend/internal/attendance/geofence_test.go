package attendance

import (
	"math"
	"testing"
)

// Satu derajat lintang sekitar 111,2 km, dan satu derajat bujur di khatulistiwa sama.
func TestDistanceM(t *testing.T) {
	monas := Point{Latitude: -6.175392, Longitude: 106.827153}
	cases := []struct {
		name string
		a, b Point
		want float64
		tol  float64
	}{
		{"titik sama", monas, monas, 0, 0.001},
		{"1 derajat lintang", Point{0, 0}, Point{1, 0}, 111_195, 5},
		{"1 derajat bujur di khatulistiwa", Point{0, 0}, Point{0, 1}, 111_195, 5},
		{"bujur menyempit di lintang tinggi", Point{60, 0}, Point{60, 1}, 55_597, 5},
		{"0,001 derajat sekitar 111 meter", monas, Point{-6.176392, 106.827153}, 111.2, 0.5},
		{"Jakarta ke Bandung sekitar 120 km", monas, Point{-6.917464, 107.619123}, 120_000, 2000},
	}
	for _, c := range cases {
		if got := DistanceM(c.a, c.b); math.Abs(got-c.want) > c.tol {
			t.Errorf("%s: %.1f m, ingin %.1f ± %.1f", c.name, got, c.want, c.tol)
		}
		if got, back := DistanceM(c.a, c.b), DistanceM(c.b, c.a); math.Abs(got-back) > 1e-6 {
			t.Errorf("%s: jarak harus simetris", c.name)
		}
	}
}

func TestRadiusArea(t *testing.T) {
	center := Point{Latitude: -6.2, Longitude: 106.8}
	area := Area{Center: center, RadiusM: 100}
	// 0,0009 derajat lintang sekitar 100,08 meter.
	cases := []struct {
		name string
		p    Point
		want bool
	}{
		{"pusat", center, true},
		{"50 meter", Point{-6.20045, 106.8}, true},
		{"99 meter", Point{-6.20089, 106.8}, true},
		{"101 meter", Point{-6.20091, 106.8}, false},
		{"jauh", Point{-7.0, 110.0}, false},
	}
	for _, c := range cases {
		if got := area.Contains(c.p); got != c.want {
			t.Errorf("%s: %v (%.1f m)", c.name, got, DistanceM(center, c.p))
		}
	}
}

func TestPolygonArea(t *testing.T) {
	// Bentuk L: kotak kanan atas kosong.
	l := Area{
		Center:  Point{-6.2, 106.8}, // pusat harus diabaikan bila ada poligon
		RadiusM: 100_000,
		Polygon: []Point{
			{0, 0}, {0, 2}, {1, 2}, {1, 1}, {2, 1}, {2, 0},
		},
	}
	cases := []struct {
		name string
		p    Point
		want bool
	}{
		{"kaki bawah", Point{0.5, 1.5}, true},
		{"kaki kiri", Point{1.5, 0.5}, true},
		{"sudut dalam kosong", Point{1.5, 1.5}, false},
		{"di luar", Point{3, 3}, false},
		{"di luar kiri", Point{0.5, -0.5}, false},
		{"pusat radius tidak berlaku", Point{-6.2, 106.8}, false},
	}
	for _, c := range cases {
		if got := l.Contains(c.p); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestFailsClosed(t *testing.T) {
	good := Point{-6.2, 106.8}
	area := Area{Center: good, RadiusM: 100}
	nan := math.NaN()
	inf := math.Inf(1)
	for name, p := range map[string]Point{
		"lintang NaN":  {nan, 106.8},
		"bujur NaN":    {-6.2, nan},
		"lintang inf":  {inf, 106.8},
		"bujur -inf":   {-6.2, -inf},
		"lintang >90":  {91, 106.8},
		"bujur >180":   {-6.2, 181},
		"lintang <-90": {-91, 106.8},
		"bujur <-180":  {-6.2, -181},
	} {
		if area.Contains(p) {
			t.Errorf("%s diterima", name)
		}
		if (Area{Polygon: []Point{{0, 0}, {0, 2}, {2, 2}}}).Contains(p) {
			t.Errorf("%s diterima poligon", name)
		}
	}
	for name, a := range map[string]Area{
		"radius nol":        {Center: good},
		"radius negatif":    {Center: good, RadiusM: -5},
		"pusat tidak valid": {Center: Point{nan, nan}, RadiusM: 100},
		"poligon dua titik": {Polygon: []Point{{0, 0}, {1, 1}}},
		"poligon rusak":     {Polygon: []Point{{0, 0}, {nan, 2}, {2, 2}}},
	} {
		if a.Contains(good) || a.Contains(Point{0.5, 0.5}) {
			t.Errorf("%s menerima titik", name)
		}
	}
}
