package integrity

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestAttendanceHashCanonical(t *testing.T) {
	when := time.Date(2026, 11, 7, 8, 2, 3, 0, time.FixedZone("WIB", 7*3600))
	nonce, selfie := strings.Repeat("a", 64), strings.Repeat("b", 64)
	got, err := AttendanceHash(nonce, -6.2001, 106.8166, when, selfie)
	if err != nil || got != "dbAzmlRTN__s4xBRepwpPHwwELp5UEH9JukycsqroJE" {
		t.Fatalf("hash %s, %v", got, err)
	}
	other, err := AttendanceHash(nonce, -6.2001, 106.816601, when, selfie)
	if err != nil || other == got {
		t.Fatalf("lokasi lain harus punya hash lain: %v", err)
	}
	utc, err := AttendanceHash(nonce, -6.2001, 106.8166, when.UTC(), selfie)
	if err != nil || utc != got {
		t.Fatal("waktu yang sama harus menghasilkan hash sama")
	}
}

func TestAttendanceHashRejectsInvalidInputs(t *testing.T) {
	nonce, selfie := strings.Repeat("a", 64), strings.Repeat("b", 64)
	when := time.Now()
	for _, tc := range []struct {
		nonce    string
		lat, lon float64
		at       time.Time
		selfie   string
	}{
		{"bad", 1, 1, when, selfie}, {strings.ToUpper(nonce), 1, 1, when, selfie},
		{nonce, math.NaN(), 1, when, selfie}, {nonce, 91, 1, when, selfie},
		{nonce, 1, math.Inf(1), when, selfie}, {nonce, 1.0000001, 1, when, selfie},
		{nonce, 1, 1, time.Time{}, selfie}, {nonce, 1, 1, when, "bad"},
	} {
		if _, err := AttendanceHash(tc.nonce, tc.lat, tc.lon, tc.at, tc.selfie); err == nil {
			t.Errorf("input tidak valid diterima: %+v", tc)
		}
	}
}

func TestEnrollmentHashBindsSamplesAndConsent(t *testing.T) {
	when := time.Date(2026, 11, 7, 1, 2, 3, 0, time.UTC)
	nonce := strings.Repeat("a", 64)
	samples := []string{strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)}
	hash, err := EnrollmentHash(nonce, -6.2001, 106.8166, when, "2026-11", "blink", samples)
	if err != nil || hash != "WbsmcNuVZn1opBr6SE-vjCS0jbMuTe5anTSc_XapF20" {
		t.Fatalf("hash pendaftaran %s: %v", hash, err)
	}
	samples[0], samples[1] = samples[1], samples[0]
	changed, err := EnrollmentHash(nonce, -6.2001, 106.8166, when, "2026-11", "blink", samples)
	if err != nil || changed == hash {
		t.Fatal("urutan sampel harus terikat")
	}
	if _, err := EnrollmentHash(nonce, -6.2001, 106.8166, when, "2026|11", "blink", samples); err == nil {
		t.Fatal("pemisah di consent harus ditolak")
	}
	if _, err := EnrollmentHash(nonce, -6.2001, 106.8166, when, "2026-11", "lain", samples); err == nil {
		t.Fatal("gerakan tidak valid harus ditolak")
	}
}
