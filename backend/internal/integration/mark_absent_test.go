package integration

import (
	"context"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/attendance"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

// statusesBetween mengembalikan status presensi per tanggal (YYYY-MM-DD) untuk satu Pejuang.
func statusesBetween(t *testing.T, employeeID, from, to string) map[string]string {
	t.Helper()
	rows, err := ownerPool.Query(context.Background(), `SELECT to_char(work_date, 'YYYY-MM-DD'), status::text
		FROM attendances WHERE employee_id = $1 AND work_date BETWEEN $2::date AND $3::date`, employeeID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var day, status string
		if err := rows.Scan(&day, &status); err != nil {
			t.Fatal(err)
		}
		out[day] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func wantStatuses(t *testing.T, name string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %v, ingin %v", name, got, want)
		return
	}
	for day, status := range want {
		if got[day] != status {
			t.Errorf("%s: %v, ingin %v", name, got, want)
			return
		}
	}
}

func TestMarkAbsent(t *testing.T) {
	fresh(t, options{})
	ctx := context.Background()

	// Pejuang aktif berakun aktif. Data dipasang di Januari 2020, di luar jangkauan simulasi presensi.
	rows, err := ownerPool.Query(ctx, `SELECT e.id::text FROM employees e JOIN users u ON u.employee_id = e.id
		WHERE e.deleted_at IS NULL AND e.status = 'active' ORDER BY e.nip LIMIT 9`)
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
	if len(ids) < 9 {
		t.Fatalf("Pejuang seed hanya %d", len(ids))
	}
	plain, weekend, onLeave, joined, exited, inactive, wfa, reviewed, noShift := ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6], ids[7], ids[8]

	var weekday, weekendShift, sdmUser string
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time, work_days)
		VALUES ('Uji Alpha', '08:00', '16:00', '{1,2,3,4,5}') RETURNING id::text`).Scan(&weekday); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(ctx, `INSERT INTO shifts (name, start_time, end_time, work_days)
		VALUES ('Uji Akhir Pekan', '08:00', '16:00', '{6,7}') RETURNING id::text`).Scan(&weekendShift); err != nil {
		t.Fatal(err)
	}
	if err := ownerPool.QueryRow(ctx, `SELECT id::text FROM users WHERE role = 'sdm' LIMIT 1`).Scan(&sdmUser); err != nil {
		t.Fatal(err)
	}
	exec(t, `UPDATE employees SET default_shift_id = $2, join_date = '2019-01-01', exit_date = NULL WHERE id = ANY($1::uuid[])`, ids, weekday)
	exec(t, `UPDATE users SET is_active = true WHERE employee_id = ANY($1::uuid[])`, ids)
	exec(t, `DELETE FROM attendances WHERE employee_id = ANY($1::uuid[]) AND work_date < '2021-01-01'`, ids)
	exec(t, `DELETE FROM leave_requests WHERE employee_id = ANY($1::uuid[])`, ids)
	exec(t, `UPDATE employees SET default_shift_id = $2 WHERE id = $1`, weekend, weekendShift)
	exec(t, `UPDATE employees SET default_shift_id = NULL WHERE id = $1`, noShift)
	exec(t, `UPDATE employees SET join_date = '2020-01-09' WHERE id = $1`, joined)
	exec(t, `UPDATE employees SET exit_date = '2020-01-07' WHERE id = $1`, exited)
	exec(t, `UPDATE users SET is_active = false WHERE employee_id = $1`, inactive)
	// Rabu 8 Januari 2020 libur.
	exec(t, `INSERT INTO holidays (date, name) VALUES ('2020-01-08', 'Libur uji') ON CONFLICT (date) DO NOTHING`)
	exec(t, `INSERT INTO attendances (employee_id, work_date, status, clock_in_at) VALUES ($1, '2020-01-07', 'present', '2020-01-07 08:00+07')`, plain)
	exec(t, `INSERT INTO attendances (employee_id, work_date, status, clock_in_at) VALUES ($1, '2020-01-09', 'review', '2020-01-09 08:00+07')`, reviewed)
	// Cuti tercatat di presensi pada tanggal 6, dan cuti disetujui 9-10 yang belum tercatat.
	exec(t, `INSERT INTO attendances (employee_id, work_date, status) VALUES ($1, '2020-01-06', 'leave')`, onLeave)
	exec(t, `INSERT INTO leave_requests (employee_id, leave_type_id, start_date, end_date, days, reason, status)
		SELECT $1, id, '2020-01-09', '2020-01-10', 2, 'Uji', 'approved' FROM leave_types WHERE code = 'annual'`, onLeave)
	// Cuti yang masih menunggu tidak membebaskan.
	exec(t, `INSERT INTO leave_requests (employee_id, leave_type_id, start_date, end_date, days, reason, status)
		SELECT $1, id, '2020-01-07', '2020-01-07', 1, 'Uji', 'pending' FROM leave_types WHERE code = 'annual'`, onLeave)
	exec(t, `INSERT INTO employee_work_arrangements (employee_id, mode, starts_on, reason, created_by)
		VALUES ($1, 'wfa', '2019-12-01', 'Uji', $2)`, wfa, sdmUser)

	at := func(day, hour, minute int) time.Time {
		return time.Date(2020, time.January, day, hour, minute, 0, 0, pgconv.WIB)
	}
	// Sabtu 11 Januari pukul 03.00: Jumat tanggal 10 belum tertutup.
	if _, err := attendance.MarkAbsent(ctx, appPool, at(11, 3, 0), 7); err != nil {
		t.Fatal(err)
	}
	wantStatuses(t, "sebelum cutoff", statusesBetween(t, plain, "2020-01-01", "2020-01-31"), map[string]string{
		"2020-01-03": "absent", "2020-01-06": "absent", "2020-01-07": "present", "2020-01-09": "absent",
	})

	// Setelah cutoff, hari Jumat ikut tercatat. Menjalankan untuk hari Senin 13 Januari mengejar akhir pekan.
	if _, err := attendance.MarkAbsent(ctx, appPool, at(11, 4, 20), 7); err != nil {
		t.Fatal(err)
	}
	if _, err := attendance.MarkAbsent(ctx, appPool, at(14, 4, 20), 3); err != nil {
		t.Fatal(err)
	}
	all := map[string]string{"2020-01-03": "absent", "2020-01-06": "absent", "2020-01-07": "absent", "2020-01-09": "absent", "2020-01-10": "absent", "2020-01-13": "absent"}
	without := func(days ...string) map[string]string {
		out := map[string]string{}
		for k, v := range all {
			out[k] = v
		}
		for _, d := range days {
			delete(out, d)
		}
		return out
	}
	with := func(base map[string]string, day, status string) map[string]string {
		base[day] = status
		return base
	}
	wantStatuses(t, "biasa", statusesBetween(t, plain, "2020-01-01", "2020-01-31"), with(without(), "2020-01-07", "present"))
	wantStatuses(t, "shift akhir pekan", statusesBetween(t, weekend, "2020-01-01", "2020-01-31"), map[string]string{
		"2020-01-04": "absent", "2020-01-05": "absent", "2020-01-11": "absent", "2020-01-12": "absent",
	})
	wantStatuses(t, "cuti", statusesBetween(t, onLeave, "2020-01-01", "2020-01-31"),
		with(without("2020-01-09", "2020-01-10"), "2020-01-06", "leave"))
	wantStatuses(t, "belum bergabung", statusesBetween(t, joined, "2020-01-01", "2020-01-31"), without("2020-01-03", "2020-01-06", "2020-01-07"))
	wantStatuses(t, "sudah keluar", statusesBetween(t, exited, "2020-01-01", "2020-01-31"), map[string]string{"2020-01-03": "absent", "2020-01-06": "absent", "2020-01-07": "absent"})
	wantStatuses(t, "akun nonaktif", statusesBetween(t, inactive, "2020-01-01", "2020-01-31"), map[string]string{})
	wantStatuses(t, "WFA tetap wajib presensi", statusesBetween(t, wfa, "2020-01-01", "2020-01-31"), without())
	wantStatuses(t, "menunggu review tidak ditimpa", statusesBetween(t, reviewed, "2020-01-01", "2020-01-31"), with(without(), "2020-01-09", "review"))
	wantStatuses(t, "tanpa shift", statusesBetween(t, noShift, "2020-01-01", "2020-01-31"), map[string]string{})

	var shiftOK bool
	if err := ownerPool.QueryRow(ctx, `SELECT shift_id = $2 FROM attendances WHERE employee_id = $1 AND work_date = '2020-01-06'`, plain, weekday).Scan(&shiftOK); err != nil || !shiftOK {
		t.Errorf("alpha harus mencatat shift: %v", err)
	}

	// Idempoten: menjalankan ulang tidak menambah atau mengubah apa pun.
	for _, now := range []time.Time{at(11, 4, 20), at(14, 4, 20)} {
		n, err := attendance.MarkAbsent(ctx, appPool, now, 7)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("jalankan ulang %s mencatat %d baris", now, n)
		}
	}
	wantStatuses(t, "biasa setelah diulang", statusesBetween(t, plain, "2020-01-01", "2020-01-31"), with(without(), "2020-01-07", "present"))

	// Cuti yang disetujui belakangan tetap menimpa alpha.
	exec(t, `INSERT INTO attendances (employee_id, work_date, status) VALUES ($1, '2020-01-13', 'leave')
		ON CONFLICT (employee_id, work_date) DO UPDATE SET status = 'leave' WHERE attendances.clock_in_at IS NULL`, plain)
	if got := statusesBetween(t, plain, "2020-01-13", "2020-01-13")["2020-01-13"]; got != "leave" {
		t.Errorf("alpha harus bisa ditimpa cuti, dapat %q", got)
	}

	if _, err := attendance.MarkAbsent(ctx, appPool, at(14, 4, 20), 0); err == nil {
		t.Error("days=0 harus ditolak")
	}
}
