package integration

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"
)

func TestReviewSelfieAuthorizationAuditAndRetention(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	f.enroll(t, enrollSamples(), nil).want(t, http.StatusCreated)
	photoBytes := photo("1", 0.1)
	in := data[clockInResp](t, f.faceClock(t, "clock_in", photoBytes, nil).want(t, http.StatusCreated))
	path := "/v1/admin/attendance/" + in.AttendanceID + "/selfie/clock_in"
	// Hanya presensi review dan hanya peninjau boleh melihat.
	f.sdm.do(http.MethodGet, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	exec(t, `UPDATE attendances SET status='review' WHERE id=$1`, in.AttendanceID)
	f.hp.do(http.MethodGet, path, nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	r := f.sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK)
	if !bytes.Equal(r.body, photoBytes) || r.header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("foto atau header privasi tidak sesuai")
	}
	if countAudit(t, "face.capture_viewed") != 1 {
		t.Fatal("akses selfie harus diaudit")
	}
	f.sdm.do(http.MethodGet, "/v1/admin/attendance/"+in.AttendanceID+"/selfie/clock_out", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	exec(t, `UPDATE face_captures SET created_at=$1 WHERE attendance_id=$2`, f.clock.Now().Add(-91*24*time.Hour), in.AttendanceID)
	f.sdm.do(http.MethodGet, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	var count int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action='face.capture_viewed'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit tidak sesuai: %d, %v", count, err)
	}
}
