package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
)

type fakePlayVerifier struct {
	err   error
	token string
	hash  string
	calls int
}

func TestPlayIntegrityEnforcedForEnrollment(t *testing.T) {
	verifier := &fakePlayVerifier{err: integrity.ErrInvalid}
	f := newClockFixture(t, options{strictIntegrity: true, playVerifier: verifier}, "08:00", "17:00")
	proof := map[string]any{"captured_at": f.clock.Now().UTC().Format(time.RFC3339), "integrity_token": "token"}
	f.enroll(t, enrollSamples(), proof).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	if verifier.calls != 1 || verifier.token != "token" {
		t.Fatal("token pendaftaran harus diperiksa")
	}
	verifier.err = nil
	f.enroll(t, enrollSamples(), proof).want(t, http.StatusCreated)
	enrollmentHash := verifier.hash
	f.faceClock(t, "clock_in", photo("1", 0.1), map[string]any{"integrity_token": "clock"}).want(t, http.StatusCreated)
	if verifier.hash == enrollmentHash {
		t.Fatal("hash absen harus berbeda dari hash pendaftaran")
	}
}

func (v *fakePlayVerifier) Verify(_ context.Context, token, hash string) error {
	v.token, v.hash = token, hash
	v.calls++
	return v.err
}

func TestPlayIntegrityEnforcedForClock(t *testing.T) {
	verifier := &fakePlayVerifier{err: integrity.ErrInvalid}
	f := newClockFixture(t, options{strictIntegrity: true, playVerifier: verifier}, "08:00", "17:00")
	first := photo("1", 0.1)
	f.faceClock(t, "clock_in", first, map[string]any{"integrity_token": "invalid"}).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
	if verifier.calls != 1 || verifier.token != "invalid" || len(verifier.hash) != 43 {
		t.Fatal("verifier harus menerima token dan hash yang dihitung server")
	}
	firstHash := verifier.hash
	verifier.err = nil
	// Bukti valid belum cukup bila wajah belum terdaftar.
	f.faceClock(t, "clock_in", photo("1", 0.1), map[string]any{"integrity_token": "valid"}).wantError(t, http.StatusUnprocessableEntity, "FACE_NOT_ENROLLED")
	if verifier.calls != 2 || verifier.hash == firstHash {
		t.Fatal("hash harus terikat pada challenge dan foto")
	}
	verifier.err = errors.New("network failure")
	f.faceClock(t, "clock_in", photo("1", 0.1), map[string]any{"integrity_token": "valid"}).wantError(t, http.StatusUnprocessableEntity, "ATTENDANCE_BLOCKED")
}
