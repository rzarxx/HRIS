package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

type reportDetailRow struct {
	EmployeeID    string `json:"employee_id"`
	WorkDays      int    `json:"work_days"`
	Present       int    `json:"present"`
	Leave         int    `json:"leave"`
	LeaveQuota    int    `json:"leave_quota"`
	LeaveNonQuota int    `json:"leave_non_quota"`
	LeaveOther    int    `json:"leave_other"`
	LeaveUnpaid   int    `json:"leave_unpaid"`
	LeaveByType   []struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		QuotaKind string `json:"quota_kind"`
		Days      int    `json:"days"`
	} `json:"leave_by_type"`
	Absent         int `json:"absent"`
	WorkedMinutes  int `json:"worked_minutes"`
	MissedClockOut int `json:"missed_clock_out"`
}

func reportRowFor(t *testing.T, c *client, query, employeeID string) reportDetailRow {
	t.Helper()
	for _, r := range data[[]reportDetailRow](t, c.do(http.MethodGet, "/v1/admin/reports/attendance?"+query, nil).want(t, http.StatusOK)) {
		if r.EmployeeID == employeeID {
			return r
		}
	}
	t.Fatalf("Pejuang tidak ada di rekap %s", query)
	return reportDetailRow{}
}

// Rekap memisahkan cuti per jenis, menjumlahkan jam kerja, dan menghitung lupa presensi pulang.
func TestAttendanceReportDetail(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	ctx := context.Background()
	id := employeeInSubdivision(t, "OPS-GDG")
	exec(t, `DELETE FROM attendances WHERE employee_id = $1`, id)
	exec(t, `DELETE FROM leave_requests WHERE employee_id = $1`, id)

	var regular, night string
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time) VALUES ('Uji Reguler', '08:00', '16:00') RETURNING id::text`).Scan(&regular); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time) VALUES ('Uji Malam', '22:00', '06:00') RETURNING id::text`).Scan(&night); err != nil {
		t.Fatal(err)
	}
	clock := func(date, shift, status, in string, out, worked any) {
		exec(t, `INSERT INTO attendances (employee_id, work_date, shift_id, status, clock_in_at, clock_out_at, worked_minutes)
			VALUES ($1, $2::date, $3, $4::att_status, $5::timestamptz, $6::timestamptz, $7)`, id, date, shift, status, in, out, worked)
	}
	// Maret 2020: dua hari lengkap, dua lupa pulang (shift reguler dan malam), dan satu alpha hasil
	// Review Presensi yang tidak dihitung lupa pulang.
	clock("2020-03-02", regular, "present", "2020-03-02 08:00+07", "2020-03-02 16:00+07", 480)
	clock("2020-03-03", regular, "late", "2020-03-03 08:30+07", "2020-03-03 16:00+07", 450)
	clock("2020-03-04", regular, "present", "2020-03-04 08:00+07", nil, nil)
	clock("2020-03-05", night, "present", "2020-03-05 22:00+07", nil, nil)
	clock("2020-03-12", regular, "absent", "2020-03-12 08:00+07", nil, nil)
	// Cuti: tahunan (bersaldo) 2 hari, sakit (tanpa saldo) 1 hari, satu hari leave tanpa pengajuan, dan
	// satu hari leave yang pengajuannya masih menunggu.
	for _, d := range []string{"2020-03-06", "2020-03-09", "2020-03-10", "2020-03-11", "2020-03-13"} {
		exec(t, `INSERT INTO attendances (employee_id, work_date, status) VALUES ($1, $2::date, 'leave')`, id, d)
	}
	leave := func(code, from, to, status string) {
		exec(t, `INSERT INTO leave_requests (employee_id, leave_type_id, start_date, end_date, days, reason, status)
			SELECT $1, id, $2::date, $3::date, 1, 'Uji', $4::request_status FROM leave_types WHERE code = $5`, id, from, to, status, code)
	}
	exec(t, `UPDATE leave_types SET paid = false WHERE code = 'sick'`)
	leave("annual", "2020-03-06", "2020-03-09", "approved")
	leave("sick", "2020-03-10", "2020-03-10", "approved")
	leave("annual", "2020-03-13", "2020-03-13", "pending")
	leave("sick", "2020-03-11", "2020-03-11", "rejected")

	r := reportRowFor(t, sdm, "year=2020&month=3", id)
	if r.Leave != 5 || r.LeaveQuota != 2 || r.LeaveNonQuota != 1 || r.LeaveOther != 2 || r.LeaveUnpaid != 1 {
		t.Errorf("cuti: %+v", r)
	}
	byCode := map[string]int{}
	for _, lt := range r.LeaveByType {
		byCode[lt.Code] = lt.Days
		if (lt.Code == "annual") != (lt.QuotaKind == "statutory") || lt.Name == "" {
			t.Errorf("jenis cuti: %+v", lt)
		}
	}
	if len(byCode) != 2 || byCode["annual"] != 2 || byCode["sick"] != 1 {
		t.Errorf("cuti per jenis: %+v", r.LeaveByType)
	}
	if r.WorkedMinutes != 930 || r.MissedClockOut != 2 || r.Absent != 1 {
		t.Errorf("jam kerja/lupa pulang: %+v", r)
	}
	// Mode kerja hanya menghitung presensi masuk, jadi tanpa cuti per jenis.
	if r := reportRowFor(t, sdm, "year=2020&month=3&work_mode=onsite", id); r.Leave != 0 || len(r.LeaveByType) != 0 || r.WorkedMinutes != 930 {
		t.Errorf("mode onsite: %+v", r)
	}

	// Presensi masuk hari ini yang belum pulang belum lewat batasnya.
	now := time.Now().In(pgconv.WIB)
	today := now.Format("2006-01-02")
	if now.Hour() < 4 {
		today = now.AddDate(0, 0, -1).Format("2006-01-02")
	}
	clock(today, regular, "present", now.Add(-time.Minute).Format(time.RFC3339), nil, nil)
	day, _ := time.Parse("2006-01-02", today)
	if r := reportRowFor(t, sdm, fmt.Sprintf("year=%d&month=%d", day.Year(), int(day.Month())), id); r.MissedClockOut != 0 {
		t.Errorf("presensi hari ini dihitung lupa pulang: %+v", r)
	}
}
