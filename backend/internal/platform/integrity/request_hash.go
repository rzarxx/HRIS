package integrity

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"
)

// AttendanceHash menghitung hash presensi dari input terverifikasi server dan nonce challenge.
// Koordinat harus tepat maksimal enam desimal. Waktu dinormalisasi UTC RFC3339Nano; hash foto dan
// nonce harus heksadesimal huruf kecil. Klien harus memakai representasi yang sama. Hash foto gerakan
// liveness (D50) ikut diikat agar fotonya tidak bisa ditukar.
func AttendanceHash(nonce string, latitude, longitude float64, capturedAt time.Time, selfieSHA256, gestureSHA256 string) (string, error) {
	fields, err := canonicalLocation(nonce, latitude, longitude, capturedAt)
	if err != nil || !isLowerHex32(selfieSHA256) || !isLowerHex32(gestureSHA256) {
		return "", ErrInvalid
	}
	return hashFields(append(fields, selfieSHA256, gestureSHA256)), nil
}

// EnrollmentHash mengikat semua sampel, foto gerakan, consent, dan gerakan ke challenge pendaftaran.
func EnrollmentHash(nonce string, latitude, longitude float64, capturedAt time.Time, consentVersion, action string, samples []string, gestureSHA256 string) (string, error) {
	fields, err := canonicalLocation(nonce, latitude, longitude, capturedAt)
	if err != nil || len(samples) < 3 || len(samples) > 5 || consentVersion == "" || strings.TrimSpace(consentVersion) != consentVersion || strings.Contains(consentVersion, "|") ||
		(action != "blink" && action != "turn_left" && action != "turn_right" && action != "smile") {
		return "", ErrInvalid
	}
	for _, sample := range samples {
		if !isLowerHex32(sample) {
			return "", ErrInvalid
		}
	}
	if !isLowerHex32(gestureSHA256) {
		return "", ErrInvalid
	}
	fields = append([]string{"face_enroll"}, fields...)
	fields = append(fields, consentVersion, action)
	fields = append(fields, samples...)
	return hashFields(append(fields, gestureSHA256)), nil
}

func canonicalLocation(nonce string, latitude, longitude float64, capturedAt time.Time) ([]string, error) {
	if !isLowerHex32(nonce) || capturedAt.IsZero() || capturedAt.UTC().Year() < 1 || capturedAt.UTC().Year() > 9999 ||
		math.IsNaN(latitude) || math.IsNaN(longitude) || math.IsInf(latitude, 0) || math.IsInf(longitude, 0) ||
		latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 ||
		latitude != math.Round(latitude*1e6)/1e6 || longitude != math.Round(longitude*1e6)/1e6 {
		return nil, ErrInvalid
	}
	if latitude == 0 {
		latitude = 0
	}
	if longitude == 0 {
		longitude = 0
	}
	return []string{
		nonce, strconv.FormatFloat(latitude, 'f', 6, 64), strconv.FormatFloat(longitude, 'f', 6, 64),
		capturedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func hashFields(fields []string) string {
	sum := sha256.Sum256([]byte(strings.Join(fields, "|")))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func isLowerHex32(s string) bool {
	if len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	data, err := hex.DecodeString(s)
	return err == nil && len(data) == 32
}
