package devsim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

const (
	demoLocationName = "Kantor Pusat (Demo)"
	demoReasonPrefix = "[Demo] "
	demoEmployees    = 8
)

// DemoSummary adalah jumlah data yang disiapkan SeedDemo.
type DemoSummary struct {
	WorkDate     time.Time
	Reviews      int
	Arrangements int
	Devices      int
}

type geoPoint struct{ lat, lon float64 }

var (
	demoOffice  = geoPoint{-6.2001, 106.8166}
	demoHome    = geoPoint{-6.4025, 106.7942} // Depok
	demoBandung = geoPoint{-6.9175, 107.6191}
	demoYogya   = geoPoint{-7.7956, 110.3695}
)

type demoReview struct {
	employee  int
	mode      string
	flexible  bool
	inMinute  int
	late      int
	in        geoPoint
	outMinute int // 0 berarti belum absen pulang
	out       geoPoint
	accuracy  float64
	score     int
	flags     []string
}

// Skor mengikuti poin di MASTER.md §5.4, jadi semuanya berada di rentang review.
var demoReviews = []demoReview{
	{employee: 0, mode: "wfa", inMinute: 8 * 60, in: demoHome, outMinute: 10 * 60, out: demoYogya, accuracy: 12, score: 40, flags: []string{"location_jump"}},
	{employee: 1, mode: "field", flexible: true, inMinute: 13*60 + 20, in: demoBandung, accuracy: 85, score: 35, flags: []string{"accuracy_low", "clock_skew"}},
	{employee: 2, mode: "onsite", inMinute: 8*60 + 25, late: 25, in: demoOffice, accuracy: 72, score: 35, flags: []string{"accuracy_low", "clock_skew"}},
	{employee: 3, mode: "onsite", inMinute: 8 * 60, in: demoOffice, outMinute: 8*60 + 30, out: demoBandung, accuracy: 9, score: 50, flags: []string{"location_jump", "new_device_recent"}},
	{employee: 4, mode: "onsite", inMinute: 7*60 + 55, in: demoOffice, outMinute: 17*60 + 2, out: demoOffice, accuracy: 15, score: 30, flags: []string{"clock_skew", "new_device_recent"}},
}

type demoArrangement struct {
	employee int
	mode     string
	flexible bool
	weekdays []int16
	reason   string
}

var demoArrangements = []demoArrangement{
	{employee: 0, mode: "wfa", weekdays: []int16{}, reason: "Bekerja dari rumah selama proyek berjalan"},
	{employee: 1, mode: "hybrid", flexible: true, weekdays: []int16{}, reason: "Dinas luar kota dan safari cabang"},
	{employee: 2, mode: "wfa", weekdays: []int16{5}, reason: "WFA setiap Jumat"},
}

type demoDevice struct {
	employee int
	label    string
	model    string
	version  string
	status   string
}

var demoDevices = []demoDevice{
	{employee: 5, label: "lama", model: "Samsung Galaxy A15", version: "Android 14", status: "approved"},
	{employee: 5, label: "baru", model: "Samsung Galaxy A35", version: "Android 15", status: "pending"},
	{employee: 6, label: "baru", model: "Xiaomi Redmi Note 13", version: "Android 14", status: "pending"},
}

func demoDeviceHash(employeeID uuid.UUID, label string) string {
	sum := sha256.Sum256([]byte("demo-device|" + employeeID.String() + "|" + label))
	return hex.EncodeToString(sum[:])
}

// lastWorkDay adalah hari kerja terakhir sebelum hari ini, jadi data demo tidak berebut dengan absen hari ini.
func lastWorkDay(now time.Time) time.Time {
	date := pgconv.WorkDate(now).AddDate(0, 0, -1)
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDate(0, 0, -1)
	}
	return date
}

// SeedDemo menyiapkan antrian Review Absen, izin kerja, dan permintaan perangkat untuk demo.
// Aman dijalankan ulang: data demo kembali ke kondisi awal, jadi demo bisa dilatih berkali-kali.
// Data lain tidak diubah, kecuali baris absen Pejuang demo pada tanggal itu.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool, now time.Time) (DemoSummary, error) {
	summary := DemoSummary{WorkDate: lastWorkDay(now)}
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		employees, err := demoEmployeeIDs(ctx, tx)
		if err != nil {
			return err
		}
		var sdmUser, shiftID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE role = 'sdm' ORDER BY created_at LIMIT 1`).Scan(&sdmUser); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM shifts ORDER BY name = 'Reguler' DESC, name LIMIT 1`).Scan(&shiftID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE employees SET default_shift_id = $1 WHERE id = ANY($2) AND default_shift_id IS NULL`, shiftID, employees); err != nil {
			return err
		}
		locationID, err := demoLocation(ctx, tx, employees)
		if err != nil {
			return err
		}
		if summary.Arrangements, err = seedDemoArrangements(ctx, tx, employees, sdmUser, now); err != nil {
			return err
		}
		if summary.Devices, err = seedDemoDevices(ctx, tx, employees, sdmUser, now); err != nil {
			return err
		}
		summary.Reviews, err = seedDemoReviews(ctx, tx, employees, shiftID, locationID, summary.WorkDate)
		return err
	})
	return summary, err
}

func demoEmployeeIDs(ctx context.Context, tx pgx.Tx) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT e.id FROM employees e JOIN users u ON u.employee_id = e.id
		WHERE u.role = 'pejuang' AND e.deleted_at IS NULL AND e.status = 'active'
		ORDER BY e.nip LIMIT $1`, demoEmployees)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, err
	}
	if len(ids) < demoEmployees {
		return nil, errors.New("Pejuang aktif kurang dari 8, jalankan seed dulu")
	}
	return ids, nil
}

func demoLocation(ctx context.Context, tx pgx.Tx, employees []uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM office_locations WHERE name = $1`, demoLocationName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO office_locations (name, latitude, longitude, radius_m)
			VALUES ($1, $2::float8::numeric, $3::float8::numeric, 150) RETURNING id`,
			demoLocationName, demoOffice.lat, demoOffice.lon).Scan(&id)
	}
	if err != nil {
		return id, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO employee_locations (employee_id, location_id)
		SELECT unnest($1::uuid[]), $2 ON CONFLICT DO NOTHING`, employees, id)
	return id, err
}

// Izin buatan pengguna tidak disentuh. Pejuang yang masih punya izin lain dilewati agar aturan
// satu izin per tanggal tetap terjaga.
func seedDemoArrangements(ctx context.Context, tx pgx.Tx, employees []uuid.UUID, sdmUser uuid.UUID, now time.Time) (int, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM employee_work_arrangements WHERE employee_id = ANY($1) AND reason LIKE $2`, employees, demoReasonPrefix+"%"); err != nil {
		return 0, err
	}
	startsOn := pgconv.WorkDate(now).AddDate(0, -1, 0)
	created := 0
	for _, a := range demoArrangements {
		tag, err := tx.Exec(ctx, `
			INSERT INTO employee_work_arrangements (employee_id, mode, flexible_hours, starts_on, weekdays, reason, created_by)
			SELECT $1, $2, $3, $4, $5, $6, $7
			WHERE NOT EXISTS (SELECT 1 FROM employee_work_arrangements WHERE employee_id = $1)`,
			employees[a.employee], a.mode, a.flexible, startsOn, a.weekdays, demoReasonPrefix+a.reason, sdmUser)
		if err != nil {
			return 0, err
		}
		created += int(tag.RowsAffected())
	}
	return created, nil
}

// Perangkat demo dihapus lalu dibuat ulang, karena latihan menyetujui atau mencabut mengubah statusnya.
func seedDemoDevices(ctx context.Context, tx pgx.Tx, employees []uuid.UUID, sdmUser uuid.UUID, now time.Time) (int, error) {
	hashes := make([]string, 0, len(demoDevices))
	for _, d := range demoDevices {
		hashes = append(hashes, demoDeviceHash(employees[d.employee], d.label))
	}
	if _, err := tx.Exec(ctx, `DELETE FROM devices WHERE device_hash = ANY($1)`, hashes); err != nil {
		return 0, err
	}
	created := 0
	for _, d := range demoDevices {
		var approvedBy *uuid.UUID
		var approvedAt *time.Time
		if d.status == "approved" {
			at := now.AddDate(0, -1, 0)
			approvedBy, approvedAt = &sdmUser, &at
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO devices (employee_id, device_hash, model, os_version, status, approved_by, approved_at, last_seen_at)
			SELECT $1, $2, $3, $4, $5::device_status, $6::uuid, $7::timestamptz, $8
			WHERE $5 <> 'approved' OR NOT EXISTS (SELECT 1 FROM devices WHERE employee_id = $1 AND status = 'approved')`,
			employees[d.employee], demoDeviceHash(employees[d.employee], d.label), d.model, d.version, d.status, approvedBy, approvedAt, now)
		if err != nil {
			return 0, err
		}
		created += int(tag.RowsAffected())
	}
	return created, nil
}

func seedDemoReviews(ctx context.Context, tx pgx.Tx, employees []uuid.UUID, shiftID, locationID uuid.UUID, date time.Time) (int, error) {
	for _, r := range demoReviews {
		flags, err := json.Marshal(r.flags)
		if err != nil {
			return 0, err
		}
		var location *uuid.UUID
		if r.mode == "onsite" {
			location = &locationID
		}
		var outAt *time.Time
		var outLat, outLon *float64
		var worked *int
		if r.outMinute > 0 {
			at := wibTime(date, r.outMinute)
			minutes := r.outMinute - r.inMinute
			outAt, outLat, outLon, worked = &at, &r.out.lat, &r.out.lon, &minutes
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO attendances (employee_id, work_date, shift_id, clock_in_at, clock_out_at, status, late_minutes, risk_score, risk_flags,
			                         work_mode, flexible_hours, location_id, worked_minutes, in_accuracy_m,
			                         in_latitude, in_longitude, out_latitude, out_longitude)
			VALUES ($1, $2, $3, $4, $5, 'review', $6, $7, $8::jsonb, $9, $10, $11::uuid, $12::int, $13::float8::numeric,
			        $14::float8::numeric, $15::float8::numeric, $16::float8::numeric, $17::float8::numeric)
			ON CONFLICT (employee_id, work_date) DO UPDATE SET
			  shift_id = EXCLUDED.shift_id, clock_in_at = EXCLUDED.clock_in_at, clock_out_at = EXCLUDED.clock_out_at, status = 'review',
			  late_minutes = EXCLUDED.late_minutes, risk_score = EXCLUDED.risk_score, risk_flags = EXCLUDED.risk_flags,
			  work_mode = EXCLUDED.work_mode, flexible_hours = EXCLUDED.flexible_hours, location_id = EXCLUDED.location_id,
			  worked_minutes = EXCLUDED.worked_minutes, in_accuracy_m = EXCLUDED.in_accuracy_m,
			  in_latitude = EXCLUDED.in_latitude, in_longitude = EXCLUDED.in_longitude,
			  out_latitude = EXCLUDED.out_latitude, out_longitude = EXCLUDED.out_longitude,
			  review_by = NULL, reviewed_at = NULL, review_note = NULL`,
			employees[r.employee], pgconv.Date(date), shiftID, wibTime(date, r.inMinute), outAt, r.late, r.score, string(flags),
			r.mode, r.flexible, location, worked, r.accuracy, r.in.lat, r.in.lon, outLat, outLon)
		if err != nil {
			return 0, err
		}
	}
	return len(demoReviews), nil
}
