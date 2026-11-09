package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

func TestRemoteFacePermission(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	path := "/v1/admin/employees/" + f.account.employeeID + "/face/remote-permission"
	outside := map[string]any{"latitude": officeLat + 0.05}
	f.enroll(t, enrollSamples(), outside).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.hp.do(http.MethodPut, path, map[string]any{"reason": "luar kota"}).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	f.sdm.do(http.MethodPut, path, map[string]any{"reason": ""}).want(t, http.StatusBadRequest)
	f.sdm.do(http.MethodPut, path, map[string]any{"reason": "luar kota"}).want(t, http.StatusNoContent)
	if countAudit(t, "face.remote_permission_granted") != 1 {
		t.Fatal("izin harus diaudit")
	}
	var oldID uuid.UUID
	if err := ownerPool.QueryRow(context.Background(), `SELECT id FROM face_remote_permissions WHERE employee_id=$1`, f.account.employeeID).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	f.sdm.do(http.MethodPut, path, map[string]any{"reason": "izin baru"}).want(t, http.StatusNoContent)
	consumed, err := store.New(appPool).ConsumeRemoteFacePermission(context.Background(), store.ConsumeRemoteFacePermissionParams{EmployeeID: uuid.MustParse(f.account.employeeID), ID: oldID, Now: pgconv.Time(f.clock.Now())})
	if err != nil || consumed != 0 {
		t.Fatalf("izin lama tidak boleh menggunakan izin pengganti: %d %v", consumed, err)
	}
	f.enroll(t, enrollSamples(), map[string]any{"latitude": officeLat + 0.05, "liveness_passed": false}).wantError(t, http.StatusUnprocessableEntity, "LIVENESS_FAILED")
	// Kegagalan tidak mengonsumsi izin, tetapi hanya satu pendaftaran berhasil.
	f.enroll(t, enrollSamples(), outside).want(t, http.StatusCreated)
	f.enroll(t, enrollSamples(), outside).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	var present bool
	if err := ownerPool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM face_remote_permissions WHERE employee_id=$1)`, f.account.employeeID).Scan(&present); err != nil || present {
		t.Fatalf("izin seharusnya habis: %v %v", present, err)
	}
	f.sdm.do(http.MethodPut, path, map[string]any{"reason": "luar kota"}).want(t, http.StatusNoContent)
	f.sdm.do(http.MethodDelete, path, nil).want(t, http.StatusNoContent)
	if countAudit(t, "face.remote_permission_revoked") != 1 {
		t.Fatal("pencabutan harus diaudit")
	}
	f.enroll(t, enrollSamples(), outside).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
	f.sdm.do(http.MethodPut, path, map[string]any{"reason": "luar kota"}).want(t, http.StatusNoContent)
	f.clock.Set(f.clock.Now().Add(25 * time.Hour))
	f.enroll(t, enrollSamples(), outside).wantError(t, http.StatusUnprocessableEntity, "GEOFENCE_OUT")
}
