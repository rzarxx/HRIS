package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

type autoClockOutRow struct {
	ID         string  `json:"id"`
	EmployeeID string  `json:"employee_id"`
	WorkDate   string  `json:"work_date"`
	ClockOutAt *string `json:"clock_out_at"`
	ShiftName  string  `json:"shift_name"`
}

type adjustedRow struct {
	Status        string  `json:"status"`
	ClockOutAt    *string `json:"clock_out_at"`
	WorkedMinutes *int32  `json:"worked_minutes"`
}

// TestAutoClockOut memeriksa penutupan lupa absen pulang oleh job harian dan koreksi SDM (D69).
func TestAutoClockOut(t *testing.T) {
	srv := fresh(t, options{})
	ctx := context.Background()
	sdm := loginSDM(t, srv)

	var day, night, sdmEmployee string
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time, work_days)
		VALUES ('Uji Pulang', '08:00', '16:00', '{1,2,3,4,5}') RETURNING id::text`).Scan(&day); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time, work_days)
		VALUES ('Uji Malam', '20:00', '04:00', '{1,2,3,4,5}') RETURNING id::text`).Scan(&night); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(ctx, `SELECT employee_id::text FROM users WHERE email = $1`, sdmEmail).Scan(&sdmEmployee); err != nil {
		t.Fatal(err)
	}
	rows, err := ownerPool.Query(ctx, `SELECT e.id::text FROM employees e JOIN users u ON u.employee_id = e.id
		WHERE e.deleted_at IS NULL AND e.status = 'active' AND u.role = 'pejuang' ORDER BY e.nip LIMIT 6`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 6 {
		t.Fatalf("Pejuang seed hanya %d", len(ids))
	}
	exec(t, `DELETE FROM attendances WHERE work_date BETWEEN '2020-02-01' AND '2020-02-29'`)
	insert := func(employee, date, clockIn, shift string) string {
		t.Helper()
		var id string
		if err := ownerPool.QueryRow(ctx, `INSERT INTO attendances (employee_id, work_date, status, clock_in_at, shift_id)
			VALUES ($1, $2::date, 'present', $3::timestamptz, NULLIF($4, '')::uuid) RETURNING id::text`, employee, date, clockIn, shift).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	dayRow := insert(ids[0], "2020-02-03", "2020-02-03 07:55+07", day)
	lateIn := insert(ids[1], "2020-02-03", "2020-02-03 17:10+07", day) // masuk setelah akhir shift
	noShift := insert(ids[2], "2020-02-03", "2020-02-03 09:00+07", "") // tanpa shift: pulang = masuk
	nightRow := insert(ids[3], "2020-02-03", "2020-02-03 19:58+07", night)
	recent := insert(ids[4], "2020-02-05", "2020-02-05 08:00+07", day) // batas tutup belum lewat
	locked := insert(ids[5], "2020-02-04", "2020-02-04 08:00+07", day)
	own := insert(sdmEmployee, "2020-02-03", "2020-02-03 08:00+07", day)
	var sdmUser string
	if err := ownerPool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, sdmEmail).Scan(&sdmUser); err != nil {
		t.Fatal(err)
	}
	// Periode yang sudah diajukan melewati 4 Februari.
	exec(t, `INSERT INTO payroll_periods (year, month, start_date, end_date, status, created_by)
		VALUES (2020, 1, '2020-02-04', '2020-02-04', 'pending_approval', $1)`, sdmUser)

	// Kamis 6 Februari 2020 pukul 03.00 WIB: batas tutup tanggal 3 dan 4 sudah lewat, tanggal 5 (04.00 esok hari) belum.
	now := time.Date(2020, time.February, 6, 3, 0, 0, 0, pgconv.WIB)
	if _, err := attendance.AutoClockOut(ctx, appPool, now); err != nil {
		t.Fatal(err)
	}
	clockOut := func(id string) (string, bool) {
		t.Helper()
		var out *time.Time
		var auto bool
		if err := ownerPool.QueryRow(ctx, `SELECT clock_out_at, auto_clock_out FROM attendances WHERE id = $1`, id).Scan(&out, &auto); err != nil {
			t.Fatal(err)
		}
		if out == nil {
			return "", auto
		}
		return out.In(pgconv.WIB).Format("2006-01-02 15:04"), auto
	}
	for id, want := range map[string]string{
		dayRow: "2020-02-03 16:00", lateIn: "2020-02-03 17:10", noShift: "2020-02-03 09:00",
		nightRow: "2020-02-04 04:00", own: "2020-02-03 16:00", recent: "", locked: "",
	} {
		got, auto := clockOut(id)
		if got != want || auto != (want != "") {
			t.Errorf("%s: pulang %q otomatis %v, ingin %q", id, got, auto, want)
		}
	}
	var worked int
	if err := ownerPool.QueryRow(ctx, `SELECT worked_minutes FROM attendances WHERE id = $1`, dayRow).Scan(&worked); err != nil || worked != 485 {
		t.Errorf("menit kerja %d (%v), ingin 485", worked, err)
	}
	// Job berikutnya tidak menutup ulang.
	if n, err := attendance.AutoClockOut(ctx, appPool, now.Add(30*time.Minute)); err != nil || n != 0 {
		t.Errorf("jalan kedua menutup %d (%v)", n, err)
	}

	list := data[[]autoClockOutRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/auto-clock-outs?per_page=100", nil).want(t, http.StatusOK))
	seen := map[string]autoClockOutRow{}
	for _, r := range list {
		seen[r.ID] = r
	}
	for _, id := range []string{dayRow, lateIn, noShift, nightRow, own} {
		if _, ok := seen[id]; !ok {
			t.Errorf("%s tidak tampil di daftar pulang otomatis", id)
		}
	}
	if seen[dayRow].ShiftName != "Uji Pulang" || seen[noShift].ShiftName != "" {
		t.Errorf("nama shift: %+v %+v", seen[dayRow], seen[noShift])
	}

	path := func(id string) string { return "/v1/admin/attendance/" + id + "/adjust" }
	sdm.do(http.MethodPost, path(dayRow), map[string]any{"action": "lainnya"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	if f := sdm.do(http.MethodPost, path(dayRow), map[string]any{"action": "absent", "note": ""}).want(t, http.StatusBadRequest).fields(); f["note"] == nil {
		t.Errorf("catatan tidak hadir: %v", f)
	}
	if f := sdm.do(http.MethodPost, path(dayRow), map[string]any{"action": "set_time", "clock_out_time": "25:00"}).want(t, http.StatusBadRequest).fields(); f["clock_out_time"] == nil {
		t.Errorf("jam tidak valid: %v", f)
	}
	sdm.do(http.MethodPost, path(own), map[string]any{"action": "confirm"}).wantError(t, http.StatusConflict, "CONFLICT_SELF_ACTION")
	sdm.do(http.MethodPost, path(recent), map[string]any{"action": "confirm"}).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")
	sdm.do(http.MethodPost, "/v1/admin/attendance/00000000-0000-0000-0000-000000000001/adjust", map[string]any{"action": "confirm"}).wantError(t, http.StatusNotFound, "NOT_FOUND")

	confirmed := data[adjustedRow](t, sdm.do(http.MethodPost, path(dayRow), map[string]any{"action": "confirm"}).want(t, http.StatusOK))
	if confirmed.Status != "present" || confirmed.WorkedMinutes == nil || *confirmed.WorkedMinutes != 485 {
		t.Errorf("sesuai: %+v", confirmed)
	}
	sdm.do(http.MethodPost, path(dayRow), map[string]any{"action": "confirm"}).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")

	// Shift malam: 03.30 lebih awal dari jam masuk, jadi esok hari.
	set := data[adjustedRow](t, sdm.do(http.MethodPost, path(nightRow), map[string]any{"action": "set_time", "clock_out_time": "03:30", "note": "Konfirmasi atasan"}).want(t, http.StatusOK))
	if got, _ := clockOut(nightRow); got != "2020-02-04 03:30" || set.WorkedMinutes == nil || *set.WorkedMinutes != 452 {
		t.Errorf("ubah jam pulang: %s %+v", got, set)
	}
	absent := data[adjustedRow](t, sdm.do(http.MethodPost, path(noShift), map[string]any{"action": "absent", "note": "Tidak bekerja"}).want(t, http.StatusOK))
	if absent.Status != "absent" {
		t.Errorf("tidak hadir: %+v", absent)
	}

	// Pulang otomatis yang tanggalnya masuk periode terkunci tidak bisa dikoreksi.
	exec(t, `UPDATE attendances SET clock_out_at = '2020-02-04 16:00+07', auto_clock_out = true WHERE id = $1`, locked)
	sdm.do(http.MethodPost, path(locked), map[string]any{"action": "confirm"}).wantError(t, http.StatusConflict, "PERIOD_LOCKED")

	if n := countAudit(t, "attendance.adjusted"); n != 3 {
		t.Errorf("audit attendance.adjusted %d, ingin 3", n)
	}
	after := data[[]autoClockOutRow](t, sdm.do(http.MethodGet, "/v1/admin/attendance/auto-clock-outs?per_page=100", nil).want(t, http.StatusOK))
	for _, r := range after {
		if r.ID == dayRow || r.ID == nightRow || r.ID == noShift {
			t.Errorf("%s masih tampil setelah dicek", r.ID)
		}
	}
	// Tanpa izin review ditolak.
	pejuang := activeAccount(t, "pejuang", "SDM-PRS")
	login(t, srv, pejuang.email, seedPassword).do(http.MethodGet, "/v1/admin/attendance/auto-clock-outs", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}
