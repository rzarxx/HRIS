package report

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
