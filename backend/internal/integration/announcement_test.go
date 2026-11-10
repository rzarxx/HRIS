package integration

import (
	"net/http"
	"testing"
	"time"
)

type myAnnouncement struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Type            string  `json:"type"`
	MinReadSeconds  int     `json:"min_read_seconds"`
	OpenedAt        *string `json:"opened_at"`
	ReadAt          *string `json:"read_at"`
	ReadWaitSeconds int     `json:"read_wait_seconds"`
}

type adminAnnouncement struct {
	ID             string `json:"id"`
	MinReadSeconds int    `json:"min_read_seconds"`
	AudienceCount  int    `json:"audience_count"`
	OpenedCount    int    `json:"opened_count"`
	ReadCount      int    `json:"read_count"`
}

type announcementReader struct {
	EmployeeID string  `json:"employee_id"`
	OpenedAt   *string `json:"opened_at"`
	ReadAt     *string `json:"read_at"`
}

func activeIDs(t *testing.T, c *client) map[string]myAnnouncement {
	t.Helper()
	out := map[string]myAnnouncement{}
	for _, a := range data[[]myAnnouncement](t, c.do(http.MethodGet, "/v1/announcements/active", nil).want(t, http.StatusOK)) {
		out[a.ID] = a
	}
	return out
}

// TestAnnouncementDeliveryAndReadTimer menguji pengumuman sampai ke Pejuang sesuai divisi dan jadwal,
// serta timer "Sudah baca" yang ditegakkan server (D27).
func TestAnnouncementDeliveryAndReadTimer(t *testing.T) {
	clock := &fakeClock{now: wibTime(2, 8, 0)}
	srv := fresh(t, options{clock: clock.Now})
	exec(t, `DELETE FROM announcements`)
	sdm := loginSDM(t, srv)
	ops, opsAcc := pejuangClient(t, srv, "OPS-GDG")
	keu, keuAcc := pejuangClient(t, srv, "KEU-AKT")

	create := func(body map[string]any) adminAnnouncement {
		t.Helper()
		return data[adminAnnouncement](t, sdm.do(http.MethodPost, "/v1/admin/announcements", body).want(t, http.StatusCreated))
	}
	start := clock.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	all := create(map[string]any{"title": "Untuk semua", "body": "Isi", "type": "popup", "starts_at": start, "min_read_seconds": 15})
	opsOnly := create(map[string]any{"title": "Operasional", "body": "Isi", "type": "banner", "starts_at": start, "division_id": opsAcc.division})
	later := create(map[string]any{"title": "Besok", "body": "Isi", "type": "banner", "starts_at": clock.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	ended := create(map[string]any{
		"title": "Selesai", "body": "Isi", "type": "banner", "starts_at": clock.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
		"ends_at": clock.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
	})
	off := create(map[string]any{"title": "Nonaktif", "body": "Isi", "type": "banner", "starts_at": start, "is_active": false})
	if opsOnly.MinReadSeconds != 10 {
		t.Errorf("jeda baca default = %d, ingin 10", opsOnly.MinReadSeconds)
	}
	sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "A", "body": "B", "type": "banner", "starts_at": start, "min_read_seconds": 121,
	}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	got := activeIDs(t, ops)
	if len(got) != 2 || got[all.ID].ID == "" || got[opsOnly.ID].ID == "" {
		t.Fatalf("pengumuman Operasional = %+v", got)
	}
	if a := got[all.ID]; a.OpenedAt != nil || a.ReadWaitSeconds != 15 {
		t.Errorf("sebelum dibuka = %+v", a)
	}
	if got := activeIDs(t, keu); len(got) != 1 || got[all.ID].ID == "" {
		t.Fatalf("pengumuman Keuangan = %+v", got)
	}
	for _, id := range []string{later.ID, ended.ID, off.ID} {
		ops.do(http.MethodPost, "/v1/announcements/"+id+"/open", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	}
	keu.do(http.MethodPost, "/v1/announcements/"+opsOnly.ID+"/open", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")

	// Belum dibuka: "Sudah baca" ditolak.
	ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/read", nil).wantError(t, http.StatusUnprocessableEntity, "READ_TOO_SOON")
	opened := data[myAnnouncement](t, ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/open", nil).want(t, http.StatusOK))
	if opened.OpenedAt == nil || opened.ReadWaitSeconds != 15 {
		t.Fatalf("setelah dibuka = %+v", opened)
	}
	clock.Set(clock.Now().Add(10 * time.Second))
	// Membuka ulang tidak memulai ulang timer.
	reopened := data[myAnnouncement](t, ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/open", nil).want(t, http.StatusOK))
	if *reopened.OpenedAt != *opened.OpenedAt || reopened.ReadWaitSeconds != 5 {
		t.Fatalf("buka ulang = %+v", reopened)
	}
	tooSoon := ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/read", nil).wantError(t, http.StatusUnprocessableEntity, "READ_TOO_SOON")
	details, _ := tooSoon.errorField("details").(map[string]any)
	if wait := details["wait_seconds"]; wait != float64(5) {
		t.Errorf("wait_seconds = %v, ingin 5", wait)
	}
	clock.Set(clock.Now().Add(5 * time.Second))
	read := data[myAnnouncement](t, ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/read", nil).want(t, http.StatusOK))
	if read.ReadAt == nil || read.ReadWaitSeconds != 0 {
		t.Fatalf("setelah dibaca = %+v", read)
	}
	again := data[myAnnouncement](t, ops.do(http.MethodPost, "/v1/announcements/"+all.ID+"/read", nil).want(t, http.StatusOK))
	if *again.ReadAt != *read.ReadAt {
		t.Error("baca ulang tidak boleh mengubah waktu baca")
	}
	keu.do(http.MethodPost, "/v1/announcements/"+all.ID+"/open", nil).want(t, http.StatusOK)

	// Rekap SDM hanya menghitung sasaran.
	detail := data[adminAnnouncement](t, sdm.do(http.MethodGet, "/v1/admin/announcements/"+all.ID, nil).want(t, http.StatusOK))
	if detail.AudienceCount < 2 || detail.OpenedCount != 2 || detail.ReadCount != 1 {
		t.Errorf("rekap semua divisi = %+v", detail)
	}
	readers := data[[]announcementReader](t, sdm.do(http.MethodGet, "/v1/admin/announcements/"+all.ID+"/reads", nil).want(t, http.StatusOK))
	if len(readers) != detail.AudienceCount {
		t.Errorf("pembaca = %d, sasaran = %d", len(readers), detail.AudienceCount)
	}
	// Belum membuka di atas, sudah membaca di bawah.
	if last := readers[len(readers)-1]; last.EmployeeID != opsAcc.employeeID || last.ReadAt == nil {
		t.Errorf("urutan pembaca, terakhir = %+v", last)
	}
	byID := map[string]announcementReader{}
	for _, r := range readers {
		byID[r.EmployeeID] = r
	}
	if r := byID[keuAcc.employeeID]; r.OpenedAt == nil || r.ReadAt != nil {
		t.Errorf("Pejuang Keuangan = %+v", r)
	}
	opsDetail := data[adminAnnouncement](t, sdm.do(http.MethodGet, "/v1/admin/announcements/"+opsOnly.ID, nil).want(t, http.StatusOK))
	if opsDetail.AudienceCount >= detail.AudienceCount || opsDetail.OpenedCount != 0 {
		t.Errorf("rekap Operasional = %+v", opsDetail)
	}
	ops.do(http.MethodGet, "/v1/admin/announcements/"+all.ID+"/reads", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	// Pengumuman yang dihapus ikut menghapus catatan bacanya.
	sdm.do(http.MethodDelete, "/v1/admin/announcements/"+all.ID, nil).want(t, http.StatusNoContent)
	if got := activeIDs(t, ops); len(got) != 1 {
		t.Errorf("setelah dihapus = %+v", got)
	}
}
