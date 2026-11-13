package report

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/store"
)

func TestCalculate(t *testing.T) {
	cases := []struct {
		name                          string
		base                          string
		absent, late                  int64
		wantAbsent, wantLate, wantNet string
	}{
		{"tanpa potongan", "5200000", 0, 0, "0", "0", "5200000"},
		{"alpha satu hari", "5200000", 1, 0, "200000", "0", "5000000"},
		{"telat dua kali", "5200000", 0, 2, "0", "10000", "5190000"},
		{"pembulatan alpha", "4250000", 1, 1, "163462", "5000", "4081538"},
		{"ubar bersih tidak negatif", "100000", 30, 10, "115385", "50000", "0"},
	}
	for _, c := range cases {
		base := decimal.RequireFromString(c.base)
		absent, late, net := Calculate(base, c.absent, c.late)
		if !absent.Equal(decimal.RequireFromString(c.wantAbsent)) ||
			!late.Equal(decimal.RequireFromString(c.wantLate)) ||
			!net.Equal(decimal.RequireFromString(c.wantNet)) {
			t.Errorf("%s: dapat alpha=%s telat=%s bersih=%s", c.name, absent, late, net)
		}
	}
}

func TestAttendanceRowCountsReview(t *testing.T) {
	got := attendanceRow(store.AttendanceReportRow{Present: 3, Late: 1, Leave: 2, Absent: 1, Review: 2, LateMinutes: 25}, nil)
	if got.WorkDays != 8 || got.Review != 2 || got.Present != 3 || got.Absent != 1 || got.Leave != 2 || got.LateMinutes != 25 {
		t.Fatalf("baris rekap = %+v", got)
	}
}

func TestAttendanceRowLeaveByType(t *testing.T) {
	annual, sick := uuid.New(), uuid.New()
	got := attendanceRow(store.AttendanceReportRow{Leave: 5, WorkedMinutes: 960, MissedClockOut: 1}, []store.AttendanceReportLeaveTypesRow{
		{LeaveTypeID: annual, Code: "annual", Name: "Cuti Tahunan", QuotaKind: store.LeaveQuotaKindStatutory, Paid: true, Days: 2},
		{LeaveTypeID: sick, Code: "sick", Name: "Sakit", QuotaKind: store.LeaveQuotaKindNone, Paid: false, Days: 1},
	})
	if got.LeaveQuota != 2 || got.LeaveNonQuota != 1 || got.LeaveOther != 2 || len(got.LeaveByType) != 2 ||
		got.LeaveByType[0].Code != "annual" || got.LeaveByType[1].QuotaKind != "none" || got.LeaveUnpaid != 1 || !got.LeaveByType[0].Paid {
		t.Fatalf("cuti per jenis = %+v", got)
	}
	if got.WorkedMinutes != 960 || got.MissedClockOut != 1 {
		t.Fatalf("jam kerja/lupa pulang = %+v", got)
	}
	if empty := attendanceRow(store.AttendanceReportRow{}, nil); empty.LeaveByType == nil {
		t.Error("leave_by_type harus array kosong, bukan null")
	}
}

func TestWibWallClock(t *testing.T) {
	got := wibWallClock(time.Date(2026, 11, 2, 21, 30, 0, 0, time.UTC)) // 04.30 WIB tanggal 3
	if want := time.Date(2026, 11, 3, 4, 30, 0, 0, time.UTC); !got.Valid || !got.Time.Equal(want) {
		t.Fatalf("jam dinding WIB = %v", got.Time)
	}
}

func TestCutoffPeriod(t *testing.T) {
	from, to := CutoffPeriod(2026, time.November)
	if !from.Equal(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v, want 2026-10-26", from)
	}
	if !to.Equal(time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %v, want 2026-11-26 (exclusive)", to)
	}
}

func TestBuildAttendanceXLSX(t *testing.T) {
	emp1 := uuid.New()
	emp2 := uuid.New()
	div1 := uuid.New()

	dOct26 := pgtype.Date{Time: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), Valid: true}
	dOct27 := pgtype.Date{Time: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), Valid: true}
	dOct28 := pgtype.Date{Time: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	dNov02 := pgtype.Date{Time: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Valid: true}

	xlsxBytes, err := BuildAttendanceXLSX(ExportData{
		Year:  2026,
		Month: time.November,
		Employees: []store.AttendanceExportEmployeesRow{
			{
				EmployeeID: emp1,
				Nip:        "P001",
				FullName:   "Muhammad Ramdan",
				DivisionID: div1,
				Division:   "SDM Dan Kepejuangan",
				WorkDays:   []int16{1, 2, 3, 4, 5, 6},
			},
			{
				EmployeeID: emp2,
				Nip:        "P002",
				FullName:   "Hilman",
				DivisionID: div1,
				Division:   "SDM Dan Kepejuangan",
				JoinDate:   dOct27, // 26 Okt -> "T"
				WorkDays:   []int16{1, 2, 3, 4, 5, 6},
			},
		},
		Records: []store.AttendanceExportRecordsRow{
			{
				EmployeeID: emp1,
				WorkDate:   dOct26,
				ClockInAt:  pgtype.Timestamptz{Time: time.Date(2026, 10, 26, 0, 28, 0, 0, time.UTC), Valid: true}, // 07:28 WIB
				ClockOutAt: pgtype.Timestamptz{Time: time.Date(2026, 10, 26, 9, 44, 0, 0, time.UTC), Valid: true}, // 16:44 WIB
				Status:     store.AttStatusPresent,
				WorkMode:   "onsite",
			},
			{
				EmployeeID:  emp1,
				WorkDate:    dOct27,
				ClockInAt:   pgtype.Timestamptz{Time: time.Date(2026, 10, 27, 0, 45, 0, 0, time.UTC), Valid: true}, // 07:45 WIB (late)
				Status:      store.AttStatusLate,
				LateMinutes: 15,
				WorkMode:    "onsite",
			},
			{
				EmployeeID: emp2,
				WorkDate:   dOct28,
				Status:     store.AttStatusAbsent,
			},
		},
		Leaves: []store.AttendanceExportLeavesRow{
			{
				EmployeeID: emp1,
				StartDate:  dNov02,
				EndDate:    dNov02,
				Code:       "SAKIT",
				Name:       "Sakit",
				QuotaKind:  store.LeaveQuotaKindNone,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildAttendanceXLSX gagal: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(xlsxBytes), int64(len(xlsxBytes)))
	if err != nil {
		t.Fatalf("buka zip xlsx: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = string(raw)
	}

	wb := files["xl/workbook.xml"]
	if !strings.Contains(wb, `name="Presensi Wajah"`) || !strings.Contains(wb, `name="Rekap Kehadiran"`) {
		t.Fatalf("workbook harus punya sheet Presensi Wajah dan Rekap Kehadiran: %s", wb)
	}

	sst := files["xl/sharedStrings.xml"]
	for _, want := range []string{
		"PERIODE OKTOBER-NOVEMBER 2026",
		"Nama Pejuang",
		"Masuk",
		"Pulang",
		"Periode (Oktober-November)",
		"Jumlah HK",
		"Keterangan",
		"Persentase",
		"SEKRETARIS WAKIL KETUA UMUM",
		"SDM DAN KEPEJUANGAN",
		"Muhammad Ramdan",
		"Hilman",
	} {
		if !strings.Contains(sst, want) {
			t.Errorf("sharedStrings kehilangan %q", want)
		}
	}

	s1 := files["xl/worksheets/sheet1.xml"]
	for _, merge := range []string{
		`<mergeCell ref="A1:BM2"/>`,
		`<mergeCell ref="A3:A6"/>`,
		`<mergeCell ref="B3:B6"/>`,
		`<mergeCell ref="C3:C6"/>`,
		`<mergeCell ref="D3:O3"/>`,
		`<mergeCell ref="P3:BM3"/>`,
		`<mergeCell ref="B7:B8"/>`,
	} {
		if !strings.Contains(s1, merge) {
			t.Errorf("sheet1 kehilangan merge %s", merge)
		}
	}

	s2 := files["xl/worksheets/sheet2.xml"]
	for _, want := range []string{
		`<mergeCell ref="D3:AH3"/>`,
		`<mergeCell ref="AI3:AI4"/>`,
		`<mergeCell ref="AJ3:AO3"/>`,
		`<mergeCell ref="AP3:AP4"/>`,
		`<mergeCell ref="B5:AI6"/>`,
		`<mergeCell ref="B7:AH7"/>`,
		`SUM(AJ8:AO8)`,
		`COUNTIF(D8:AH8,"H")`,
		`IF(AI8=0,0,AJ8/AI8)`,
	} {
		if !strings.Contains(s2, want) {
			t.Errorf("sheet2 kehilangan %s", want)
		}
	}
}
