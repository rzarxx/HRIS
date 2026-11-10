package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/devsim"
)

// Data demo harus muncul lewat API seperti data asli, dan bisa dipulihkan setelah latihan.
func TestDemoSeed(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	ctx := context.Background()
	now := wibTime(4, 12, 0) // Rabu

	summary, err := devsim.SeedDemo(ctx, ownerPool, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Reviews != 5 || summary.Today != 6 || summary.Arrangements != 3 || summary.Devices != 3 || summary.WorkDate.Format("2006-01-02") != "2026-11-03" {
		t.Errorf("ringkasan: %+v", summary)
	}

	queue := data[[]reviewRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/review", nil).want(t, http.StatusOK))
	if len(queue) != 5 {
		t.Fatalf("antrian: %+v", queue)
	}
	for _, r := range queue {
		if r.RiskScore < 30 || r.RiskScore >= 60 || len(r.RiskFlags) == 0 {
			t.Errorf("baris review: %+v", r)
		}
	}
	if got := data[[]reviewRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/review?work_mode=field", nil).want(t, http.StatusOK)); len(got) != 1 {
		t.Errorf("saring field: %+v", got)
	}
	pending := data[[]deviceRow](t, sdm.do(http.MethodGet, "/v1/admin/devices?status=pending", nil).want(t, http.StatusOK))
	if len(pending) != 2 {
		t.Errorf("perangkat menunggu: %+v", pending)
	}
	wfa := data[[]reviewRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/review?work_mode=wfa", nil).want(t, http.StatusOK))
	if len(wfa) != 1 {
		t.Fatalf("saring wfa: %+v", wfa)
	}
	arrangements := data[[]arrangementRow](t, sdm.do(http.MethodGet, "/v1/admin/employees/"+wfa[0].EmployeeID+"/work-arrangements", nil).want(t, http.StatusOK))
	if len(arrangements) != 1 {
		t.Errorf("izin: %+v", arrangements)
	}

	// Latihan: putuskan presensi dan setujui perangkat pengganti.
	sdm.do(http.MethodPost, "/v1/admin/attendance/"+queue[0].ID+"/resolve", map[string]any{"decision": "absent", "note": "Latihan demo"}).want(t, http.StatusOK)
	for _, d := range pending {
		sdm.do(http.MethodPost, "/v1/admin/devices/"+d.ID+"/approve", nil).want(t, http.StatusOK)
	}

	// Menjalankan ulang memulihkan kondisi awal tanpa menggandakan data.
	if _, err := devsim.SeedDemo(ctx, ownerPool, now); err != nil {
		t.Fatal(err)
	}
	if got := data[[]reviewRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/review", nil).want(t, http.StatusOK)); len(got) != 5 {
		t.Errorf("antrian setelah dipulihkan: %d", len(got))
	}
	if got := data[[]deviceRow](t, sdm.do(http.MethodGet, "/v1/admin/devices?status=pending", nil).want(t, http.StatusOK)); len(got) != 2 {
		t.Errorf("perangkat setelah dipulihkan: %d", len(got))
	}
	var reviewBy, locations int
	if err := ownerPool.QueryRow(ctx, `SELECT count(review_by) FROM attendances WHERE work_date = '2026-11-03' AND status = 'review'`).Scan(&reviewBy); err != nil {
		t.Fatal(err)
	}
	if reviewBy != 0 {
		t.Errorf("catatan keputusan lama tersisa: %d", reviewBy)
	}
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM office_locations WHERE name LIKE '%(Demo)'`).Scan(&locations); err != nil {
		t.Fatal(err)
	}
	if locations != 1 {
		t.Errorf("lokasi demo: %d", locations)
	}
}

// Izin buatan pengguna tidak boleh hilang, dan aturan satu izin per tanggal tetap terjaga.
func TestDemoSeedKeepsUserArrangements(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	ctx := context.Background()
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC) // Senin, jadi tanggalnya Jumat sebelumnya

	summary, err := devsim.SeedDemo(ctx, ownerPool, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.WorkDate.Format("2006-01-02") != "2026-11-06" {
		t.Errorf("hari kerja terakhir: %s", summary.WorkDate.Format("2006-01-02"))
	}
	queue := data[[]reviewRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/review?work_mode=wfa", nil).want(t, http.StatusOK))
	if len(queue) != 1 {
		t.Fatalf("antrian: %+v", queue)
	}
	path := "/v1/admin/employees/" + queue[0].EmployeeID + "/work-arrangements"
	list := data[[]arrangementRow](t, sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK))
	sdm.do(http.MethodDelete, path+"/"+list[0].ID, nil).want(t, http.StatusNoContent)
	sdm.do(http.MethodPost, path, map[string]any{"mode": "hybrid", "starts_on": "2026-11-02", "reason": "Izin dari SDM"}).want(t, http.StatusCreated)

	again, err := devsim.SeedDemo(ctx, ownerPool, now)
	if err != nil {
		t.Fatal(err)
	}
	if again.Arrangements != 2 {
		t.Errorf("izin demo: %d", again.Arrangements)
	}
	kept := sdm.do(http.MethodGet, path, nil).want(t, http.StatusOK)
	if !strings.Contains(string(kept.body), "Izin dari SDM") || strings.Contains(string(kept.body), "[Demo]") {
		t.Errorf("izin pengguna berubah: %s", kept.body)
	}
}
