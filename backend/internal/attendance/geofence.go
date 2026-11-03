// Package attendance memuat aturan presensi. Geofence dihitung di sini, tanpa PostGIS (D22):
// tiap Pejuang hanya punya sedikit lokasi, jadi tidak perlu indeks spasial.
package attendance

import "math"

const earthRadiusM = 6_371_008.8 // radius rata-rata bumi (IUGG)

// Point adalah koordinat WGS84 dalam derajat desimal.
type Point struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Valid memeriksa rentang koordinat; NaN dan tak hingga ikut ditolak.
func (p Point) Valid() bool {
	return p.Latitude >= -90 && p.Latitude <= 90 && p.Longitude >= -180 && p.Longitude <= 180
}

// Area adalah batas satu lokasi kantor. Poligon, bila ada, menggantikan radius.
type Area struct {
	Center  Point
	RadiusM int
	Polygon []Point
}

// DistanceM menghitung jarak dua titik dengan rumus Haversine, dalam meter.
func DistanceM(a, b Point) float64 {
	lat1 := a.Latitude * math.Pi / 180
	lat2 := b.Latitude * math.Pi / 180
	dLat := lat2 - lat1
	dLon := (b.Longitude - a.Longitude) * math.Pi / 180
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dLon/2), 2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(h)))
}

// Contains menjawab apakah titik berada di dalam area. Masukan yang tidak valid selalu
// ditolak (fail closed, MASTER.md §1.6).
func (a Area) Contains(p Point) bool {
	if !p.Valid() {
		return false
	}
	if len(a.Polygon) > 0 {
		return insidePolygon(a.Polygon, p)
	}
	if !a.Center.Valid() || a.RadiusM <= 0 {
		return false
	}
	return DistanceM(a.Center, p) <= float64(a.RadiusM)
}

// insidePolygon memakai ray casting. Bidang datar cukup akurat untuk area seukuran kantor.
func insidePolygon(polygon []Point, p Point) bool {
	if len(polygon) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		a, b := polygon[i], polygon[j]
		if !a.Valid() || !b.Valid() {
			return false
		}
		crosses := (a.Latitude > p.Latitude) != (b.Latitude > p.Latitude)
		if crosses && p.Longitude < (b.Longitude-a.Longitude)*(p.Latitude-a.Latitude)/(b.Latitude-a.Latitude)+a.Longitude {
			inside = !inside
		}
	}
	return inside
}
