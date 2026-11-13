package report

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

var indonesianMonthsUpper = [...]string{
	"",
	"JANUARI", "FEBRUARI", "MARET", "APRIL", "MEI", "JUNI",
	"JULI", "AGUSTUS", "SEPTEMBER", "OKTOBER", "NOVEMBER", "DESEMBER",
}

var indonesianMonthsTitle = [...]string{
	"",
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

var indonesianWeekdays = [...]string{
	time.Sunday:    "Minggu",
	time.Monday:    "Senin",
	time.Tuesday:   "Selasa",
	time.Wednesday: "Rabu",
	time.Thursday:  "Kamis",
	time.Friday:    "Jumat",
	time.Saturday:  "Sabtu",
}

// CutoffPeriod mengembalikan rentang cutoff SDM: tanggal 26 bulan sebelumnya (inklusif)
// sampai tanggal 26 bulan berjalan (eksklusif, sehingga hari terakhir adalah tanggal 25).
func CutoffPeriod(year int, month time.Month) (from, to time.Time) {
	from = time.Date(year, month-1, 26, 0, 0, 0, 0, time.UTC)
	to = time.Date(year, month, 26, 0, 0, 0, 0, time.UTC)
	return from, to
}

// ExportData memuat seluruh data yang dibutuhkan untuk menyusun workbook XLSX format SDM.
type ExportData struct {
	Year         int
	Month        time.Month
	From, To     time.Time
	Employees    []store.AttendanceExportEmployeesRow
	Records      []store.AttendanceExportRecordsRow
	Leaves       []store.AttendanceExportLeavesRow
	Holidays     []pgtype.Date
	Arrangements []store.AttendanceExportArrangementsRow
}

// AttendanceXLSX mengambil data cutoff 26..25 dari basis data sesuai scope dan menyusun file .xlsx.
func (s *Service) AttendanceXLSX(ctx context.Context, p auth.Principal, period Period) ([]byte, error) {
	f, err := s.filterFor(ctx, p, period)
	if err != nil {
		return nil, err
	}
	from, to := CutoffPeriod(period.Year, period.Month)
	fromPg := pgconv.Date(from)
	toPg := pgconv.Date(to)

	emps, err := s.q.AttendanceExportEmployees(ctx, store.AttendanceExportEmployeesParams{
		FromDate:            fromPg,
		ToDate:              toPg,
		WorkMode:            period.WorkMode,
		ScopeAll:            f.all,
		ScopeSubdivisionIds: f.subs,
		ScopeDivisionIds:    f.divs,
		DivisionID:          period.DivisionID,
	})
	if err != nil {
		return nil, err
	}
	recs, err := s.q.AttendanceExportRecords(ctx, store.AttendanceExportRecordsParams{
		FromDate:            fromPg,
		ToDate:              toPg,
		WorkMode:            period.WorkMode,
		ScopeAll:            f.all,
		ScopeSubdivisionIds: f.subs,
		ScopeDivisionIds:    f.divs,
		DivisionID:          period.DivisionID,
	})
	if err != nil {
		return nil, err
	}
	var leaves []store.AttendanceExportLeavesRow
	if period.WorkMode == "" {
		leaves, err = s.q.AttendanceExportLeaves(ctx, store.AttendanceExportLeavesParams{
			FromDate:            fromPg,
			ToDate:              toPg,
			ScopeAll:            f.all,
			ScopeSubdivisionIds: f.subs,
			ScopeDivisionIds:    f.divs,
			DivisionID:          period.DivisionID,
		})
		if err != nil {
			return nil, err
		}
	}
	holidays, err := s.q.AttendanceExportHolidays(ctx, store.AttendanceExportHolidaysParams{
		FromDate: fromPg,
		ToDate:   toPg,
	})
	if err != nil {
		return nil, err
	}
	arrs, err := s.q.AttendanceExportArrangements(ctx, store.AttendanceExportArrangementsParams{
		FromDate:            fromPg,
		ToDate:              toPg,
		ScopeAll:            f.all,
		ScopeSubdivisionIds: f.subs,
		ScopeDivisionIds:    f.divs,
		DivisionID:          period.DivisionID,
	})
	if err != nil {
		return nil, err
	}

	return BuildAttendanceXLSX(ExportData{
		Year:         period.Year,
		Month:        period.Month,
		From:         from,
		To:           to,
		Employees:    emps,
		Records:      recs,
		Leaves:       leaves,
		Holidays:     holidays,
		Arrangements: arrs,
	})
}

// AttendanceExport melayani GET /v1/admin/reports/attendance/export dan mengembalikan berkas .xlsx.
func (h *Handler) AttendanceExport(w http.ResponseWriter, r *http.Request) error {
	period, err := readPeriod(r)
	if err != nil {
		return err
	}
	period.WorkMode = r.URL.Query().Get("work_mode")
	if period.WorkMode != "" && period.WorkMode != "onsite" && period.WorkMode != "wfa" && period.WorkMode != "field" {
		return httpx.Validation(map[string]string{"work_mode": "Mode kerja tidak valid."})
	}
	p, _ := auth.PrincipalFrom(r.Context())
	xlsxBytes, err := h.svc.AttendanceXLSX(r.Context(), p, period)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("Rekap_Absen_Yayasan_%s_%d.xlsx", indonesianMonthsTitle[period.Month], period.Year)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(xlsxBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(xlsxBytes)
	return nil
}

// Style indices in xl/styles.xml.
const (
	styleDefault       = 0
	styleFpTitle       = 1  // #1B5E20 fill, bold white Calibri 10pt, center, thin border
	styleFpHeaderGreen = 2  // #1B5E20 fill, bold white Calibri 10pt, center, wrap, thin border
	styleFpMonthGray   = 3  // #DADADA fill, bold black Calibri 10pt, center, numFmt 17 (mmm-yy), thin border
	styleFpDateBlue    = 4  // #0D47A1 fill, bold white Calibri 10pt, center, numFmt 164 ("d"), thin border
	styleFpSubhdrGray  = 5  // #DADADA fill, bold black Calibri 10pt, center, thin border
	styleFpNoGray      = 6  // #ECECEC fill, black Calibri 10pt, center, thin border
	styleFpDivName     = 7  // #F2F2F2 fill, black Calibri 10pt, center, thin border
	styleFpEmpName     = 8  // #F2F2F2 fill, black Calibri 10pt, center, thin border
	styleFpTimeNormal  = 9  // no fill, black Calibri 10pt, center, numFmt 20 (h:mm), thin border
	styleFpTimeLate    = 10 // #FFC7CE fill, black Calibri 10pt, center, numFmt 20 (h:mm), thin border
	styleFpTextCell    = 11 // no fill, black Calibri 10pt, center, thin border

	styleManHdrBlue9   = 12 // #0D47A1 fill, bold white Arial 9pt, center, wrap, thin border
	styleManHdrBlue14  = 13 // #0D47A1 fill, bold white Arial 14pt, center, thin border
	styleManDateBlue   = 14 // #0D47A1 fill, bold white Arial 9pt, center, numFmt 164 ("d"), thin border
	styleManHdrH       = 15 // #C8C8C8 fill, bold black Arial 9pt, center, thin border
	styleManHdrS       = 16 // #F7CAAC fill, bold black Arial 9pt, center, thin border
	styleManHdrI       = 17 // #C5E0B3 fill, bold black Arial 9pt, center, thin border
	styleManHdrA       = 18 // #FF0000 fill, bold black Arial 9pt, center, thin border
	styleManHdrL       = 19 // #BDD6EE fill, bold black Arial 9pt, center, thin border
	styleManHdrC       = 20 // #FFFF00 fill, bold black Arial 9pt, center, thin border
	styleManBannerBlue = 21 // #0D47A1 fill, bold white Arial 16pt, center, thin border
	styleManDivOrange  = 22 // #F4B083 fill, bold black Arial 16pt, center, thin border
	styleManNo         = 23 // no fill, bold black Arial 9pt, center, thin border
	styleManName       = 24 // no fill, bold black Calibri 12pt, left, thin border
	styleManCodeH      = 25 // #E2EFD9 fill, black Arial 9pt, center
	styleManCodeL      = 26 // #B4C6E7 fill, black Arial 9pt, center
	styleManCodeS      = 27 // #F7CAAC fill, black Arial 9pt, center
	styleManCodeI      = 28 // #C5E0B3 fill, black Arial 9pt, center
	styleManCodeA      = 29 // #FF0000 fill, black Arial 9pt, center
	styleManCodeC      = 30 // #FFFF00 fill, black Arial 9pt, center
	styleManCodeIS     = 31 // #92D050 fill, black Arial 9pt, center
	styleManCodeT      = 32 // #0070C0 fill, black Arial 9pt, center
	styleManSumHK      = 33 // #E3F2FD fill, black Arial 9pt, center, thin border
	styleManSumH       = 34 // #C8C8C8 fill, black Arial 9pt, center, thin border
	styleManSumS       = 35 // #F7CAAC fill, black Arial 9pt, center, thin border
	styleManSumI       = 36 // #C5E0B3 fill, black Arial 9pt, center, thin border
	styleManSumA       = 37 // #FF0000 fill, black Arial 9pt, center, thin border
	styleManSumL       = 38 // #BDD6EE fill, black Arial 9pt, center, thin border
	styleManSumC       = 39 // #FFFF00 fill, black Arial 9pt, center, thin border
	styleManSumPct     = 40 // #E3F2FD fill, black Arial 9pt, center, numFmt 9 (0%), thin border
)

type empDayKey struct {
	empID uuid.UUID
	date  string
}

type divisionGroup struct {
	Name      string
	Employees []store.AttendanceExportEmployeesRow
}

// BuildAttendanceXLSX membangun berkas .xlsx dengan 2 worksheet ("Fingerprint" dan "Manual")
// yang identik 100% dengan format rekap presensi SDM.
func BuildAttendanceXLSX(data ExportData) ([]byte, error) {
	from, to := data.From, data.To
	if from.IsZero() || to.IsZero() {
		from, to = CutoffPeriod(data.Year, data.Month)
	}
	var dates []time.Time
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	numDays := len(dates)

	// Count days in first month vs second month.
	firstMonth := from.Month()
	nPrev := 0
	for _, d := range dates {
		if d.Month() == firstMonth {
			nPrev++
		}
	}
	nCurr := numDays - nPrev

	// Index records, holidays, leaves, and arrangements.
	recMap := make(map[empDayKey]store.AttendanceExportRecordsRow, len(data.Records))
	for _, r := range data.Records {
		if r.WorkDate.Valid {
			k := empDayKey{empID: r.EmployeeID, date: r.WorkDate.Time.UTC().Format("2006-01-02")}
			recMap[k] = r
		}
	}
	holidaySet := make(map[string]bool, len(data.Holidays))
	for _, h := range data.Holidays {
		if h.Valid {
			holidaySet[h.Time.UTC().Format("2006-01-02")] = true
		}
	}
	leavesByEmp := make(map[uuid.UUID][]store.AttendanceExportLeavesRow)
	for _, l := range data.Leaves {
		leavesByEmp[l.EmployeeID] = append(leavesByEmp[l.EmployeeID], l)
	}
	arrsByEmp := make(map[uuid.UUID][]store.AttendanceExportArrangementsRow)
	for _, a := range data.Arrangements {
		arrsByEmp[a.EmployeeID] = append(arrsByEmp[a.EmployeeID], a)
	}

	// Group employees by division while preserving order.
	var groups []divisionGroup
	divIdx := make(map[string]int)
	for _, e := range data.Employees {
		idx, ok := divIdx[e.Division]
		if !ok {
			idx = len(groups)
			divIdx[e.Division] = idx
			groups = append(groups, divisionGroup{Name: e.Division})
		}
		groups[idx].Employees = append(groups[idx].Employees, e)
	}

	ss := newSharedStrings()
	sheet1XML := buildSheet1Fingerprint(ss, dates, nPrev, nCurr, groups, recMap)
	sheet2XML := buildSheet2Manual(ss, dates, groups, recMap, leavesByEmp, holidaySet, arrsByEmp)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string][]byte{
		"[Content_Types].xml":        []byte(contentTypesXML),
		"_rels/.rels":                []byte(rootRelsXML),
		"docProps/core.xml":          []byte(corePropsXML),
		"xl/workbook.xml":            []byte(workbookXML),
		"xl/_rels/workbook.xml.rels": []byte(workbookRelsXML),
		"xl/styles.xml":              []byte(stylesXML),
		"xl/sharedStrings.xml":       ss.marshal(),
		"xl/worksheets/sheet1.xml":   sheet1XML,
		"xl/worksheets/sheet2.xml":   sheet2XML,
	}
	// Deterministic zip entry order.
	order := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/core.xml",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/styles.xml",
		"xl/sharedStrings.xml",
		"xl/worksheets/sheet1.xml",
		"xl/worksheets/sheet2.xml",
	}
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildSheet1Fingerprint(
	ss *sharedStrings,
	dates []time.Time,
	nPrev, nCurr int,
	groups []divisionGroup,
	recMap map[empDayKey]store.AttendanceExportRecordsRow,
) []byte {
	numDays := len(dates)
	lastColIdx := 3 + 2*numDays - 1 // 0-indexed (e.g. 64 -> BM when numDays=31)
	lastColName := excelColName(lastColIdx)

	firstDate := dates[0]
	lastDate := dates[len(dates)-1]
	periodTitle := fmt.Sprintf("PERIODE %s-%s %d",
		indonesianMonthsUpper[firstDate.Month()],
		indonesianMonthsUpper[lastDate.Month()],
		lastDate.Year(),
	)

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<cols>`)
	b.WriteString(`<col min="1" max="1" width="8.57" customWidth="1"/>`)
	b.WriteString(`<col min="2" max="2" width="27.86" customWidth="1"/>`)
	b.WriteString(`<col min="3" max="3" width="31.14" customWidth="1"/>`)
	fmt.Fprintf(&b, `<col min="4" max="%d" width="8.57" customWidth="1"/>`, lastColIdx+1)
	b.WriteString(`</cols>`)
	b.WriteString(`<sheetData>`)

	// Row 1 & Row 2: Title banner A1:<LastCol>2
	b.WriteString(`<row r="1">`)
	writeSharedStrCell(&b, "A1", styleFpTitle, ss.add(periodTitle))
	for c := 1; c <= lastColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 1), styleFpTitle)
	}
	b.WriteString(`</row>`)
	b.WriteString(`<row r="2">`)
	for c := 0; c <= lastColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 2), styleFpTitle)
	}
	b.WriteString(`</row>`)

	// Row 3: No., DIVISI, Nama Pejuang, Month headers
	b.WriteString(`<row r="3">`)
	writeSharedStrCell(&b, "A3", styleFpHeaderGreen, ss.add("No."))
	writeSharedStrCell(&b, "B3", styleFpHeaderGreen, ss.add("DIVISI"))
	writeSharedStrCell(&b, "C3", styleFpHeaderGreen, ss.add("Nama Pejuang"))
	if nPrev > 0 {
		m1 := time.Date(firstDate.Year(), firstDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		writeNumCell(&b, cellRef(3, 3), styleFpMonthGray, strconv.Itoa(excelSerialDate(m1)))
		for c := 4; c < 3+2*nPrev; c++ {
			writeBlankCell(&b, cellRef(c, 3), styleFpMonthGray)
		}
	}
	if nCurr > 0 {
		startCol := 3 + 2*nPrev
		m2 := time.Date(lastDate.Year(), lastDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		writeNumCell(&b, cellRef(startCol, 3), styleFpMonthGray, strconv.Itoa(excelSerialDate(m2)))
		for c := startCol + 1; c <= lastColIdx; c++ {
			writeBlankCell(&b, cellRef(c, 3), styleFpMonthGray)
		}
	}
	b.WriteString(`</row>`)

	// Row 4: Dates (Excel serial date with numFmt "d", merged per 2 columns)
	b.WriteString(`<row r="4">`)
	writeBlankCell(&b, "A4", styleFpHeaderGreen)
	writeBlankCell(&b, "B4", styleFpHeaderGreen)
	writeBlankCell(&b, "C4", styleFpHeaderGreen)
	for i, d := range dates {
		colM := 3 + 2*i
		colP := colM + 1
		writeNumCell(&b, cellRef(colM, 4), styleFpDateBlue, strconv.Itoa(excelSerialDate(d)))
		writeBlankCell(&b, cellRef(colP, 4), styleFpDateBlue)
	}
	b.WriteString(`</row>`)

	// Row 5: Indonesian weekday names (merged per 2 columns)
	b.WriteString(`<row r="5">`)
	writeBlankCell(&b, "A5", styleFpHeaderGreen)
	writeBlankCell(&b, "B5", styleFpHeaderGreen)
	writeBlankCell(&b, "C5", styleFpHeaderGreen)
	for i, d := range dates {
		colM := 3 + 2*i
		colP := colM + 1
		writeSharedStrCell(&b, cellRef(colM, 5), styleFpSubhdrGray, ss.add(indonesianWeekdays[d.Weekday()]))
		writeBlankCell(&b, cellRef(colP, 5), styleFpSubhdrGray)
	}
	b.WriteString(`</row>`)

	// Row 6: Masuk | Pulang
	b.WriteString(`<row r="6">`)
	writeBlankCell(&b, "A6", styleFpHeaderGreen)
	writeBlankCell(&b, "B6", styleFpHeaderGreen)
	writeBlankCell(&b, "C6", styleFpHeaderGreen)
	masukIdx := ss.add("Masuk")
	pulangIdx := ss.add("Pulang")
	for i := range dates {
		colM := 3 + 2*i
		colP := colM + 1
		writeSharedStrCell(&b, cellRef(colM, 6), styleFpSubhdrGray, masukIdx)
		writeSharedStrCell(&b, cellRef(colP, 6), styleFpSubhdrGray, pulangIdx)
	}
	b.WriteString(`</row>`)

	// Data rows starting at Row 7
	var merges []string
	merges = append(merges,
		fmt.Sprintf("A1:%s2", lastColName),
		"A3:A6",
		"B3:B6",
		"C3:C6",
	)
	if nPrev > 0 {
		merges = append(merges, fmt.Sprintf("D3:%s3", excelColName(3+2*nPrev-1)))
	}
	if nCurr > 0 {
		merges = append(merges, fmt.Sprintf("%s3:%s3", excelColName(3+2*nPrev), lastColName))
	}
	for i := range dates {
		colM := excelColName(3 + 2*i)
		colP := excelColName(3 + 2*i + 1)
		merges = append(merges, fmt.Sprintf("%s4:%s4", colM, colP))
		merges = append(merges, fmt.Sprintf("%s5:%s5", colM, colP))
	}

	dashIdx := ss.add("-")
	rowNum := 7
	seqNo := 1
	for _, g := range groups {
		divStartRow := rowNum
		for idxInDiv, emp := range g.Employees {
			fmt.Fprintf(&b, `<row r="%d">`, rowNum)
			writeNumCell(&b, cellRef(0, rowNum), styleFpNoGray, strconv.Itoa(seqNo))
			if idxInDiv == 0 {
				writeSharedStrCell(&b, cellRef(1, rowNum), styleFpDivName, ss.add(g.Name))
			} else {
				writeBlankCell(&b, cellRef(1, rowNum), styleFpDivName)
			}
			writeSharedStrCell(&b, cellRef(2, rowNum), styleFpEmpName, ss.add(emp.FullName))

			for i, d := range dates {
				colM := 3 + 2*i
				colP := colM + 1
				dateKey := d.Format("2006-01-02")
				rec, hasRec := recMap[empDayKey{empID: emp.EmployeeID, date: dateKey}]

				// Masuk cell
				if hasRec && rec.ClockInAt.Valid {
					st := styleFpTimeNormal
					if rec.Status == store.AttStatusLate || rec.LateMinutes > 0 {
						st = styleFpTimeLate
					}
					writeNumCell(&b, cellRef(colM, rowNum), st, excelTimeFraction(rec.ClockInAt.Time))
				} else if hasRec && rec.ClockOutAt.Valid {
					writeSharedStrCell(&b, cellRef(colM, rowNum), styleFpTextCell, dashIdx)
				} else {
					writeBlankCell(&b, cellRef(colM, rowNum), styleFpTimeNormal)
				}

				// Pulang cell
				if hasRec && rec.ClockOutAt.Valid {
					writeNumCell(&b, cellRef(colP, rowNum), styleFpTimeNormal, excelTimeFraction(rec.ClockOutAt.Time))
				} else if hasRec && rec.ClockInAt.Valid && rec.Status != store.AttStatusAbsent && rec.Status != store.AttStatusLeave {
					writeSharedStrCell(&b, cellRef(colP, rowNum), styleFpTextCell, dashIdx)
				} else {
					writeBlankCell(&b, cellRef(colP, rowNum), styleFpTimeNormal)
				}
			}

			b.WriteString(`</row>`)
			rowNum++
			seqNo++
		}
		divEndRow := rowNum - 1
		if divEndRow > divStartRow {
			merges = append(merges, fmt.Sprintf("B%d:B%d", divStartRow, divEndRow))
		}
	}

	b.WriteString(`</sheetData>`)

	// Merge cells
	fmt.Fprintf(&b, `<mergeCells count="%d">`, len(merges))
	for _, ref := range merges {
		fmt.Fprintf(&b, `<mergeCell ref="%s"/>`, ref)
	}
	b.WriteString(`</mergeCells>`)

	// Conditional formatting matching template
	var dateColsSqref []string
	for i := range dates {
		dateColsSqref = append(dateColsSqref, fmt.Sprintf("%s4", excelColName(3+2*i)))
	}
	sqrefDates := strings.Join(dateColsSqref, " ")
	fmt.Fprintf(&b, `<conditionalFormatting sqref="%s"><cfRule type="expression" dxfId="0" priority="1"><formula>WEEKDAY(D4,2)=7</formula></cfRule></conditionalFormatting>`, sqrefDates)
	lastDataRow := max(rowNum-1, 7)
	fmt.Fprintf(&b, `<conditionalFormatting sqref="D7:%s%d"><cfRule type="cellIs" dxfId="12" priority="2" operator="between"><formula>"7:31"</formula><formula>"16:00"</formula></cfRule></conditionalFormatting>`, lastColName, lastDataRow)

	b.WriteString(`</worksheet>`)
	return b.Bytes()
}

func buildSheet2Manual(
	ss *sharedStrings,
	dates []time.Time,
	groups []divisionGroup,
	recMap map[empDayKey]store.AttendanceExportRecordsRow,
	leavesByEmp map[uuid.UUID][]store.AttendanceExportLeavesRow,
	holidaySet map[string]bool,
	arrsByEmp map[uuid.UUID][]store.AttendanceExportArrangementsRow,
) []byte {
	numDays := len(dates)
	firstDateColIdx := 3              // D
	lastDateColIdx := 3 + numDays - 1 // AH when numDays=31
	hkColIdx := 3 + numDays           // AI
	hColIdx := 4 + numDays            // AJ
	sColIdx := 5 + numDays            // AK
	iColIdx := 6 + numDays            // AL
	aColIdx := 7 + numDays            // AM
	lColIdx := 8 + numDays            // AN
	cColIdx := 9 + numDays            // AO
	pctColIdx := 10 + numDays         // AP

	lastDateColName := excelColName(lastDateColIdx)
	hkColName := excelColName(hkColIdx)
	hColName := excelColName(hColIdx)
	cColName := excelColName(cColIdx)
	pctColName := excelColName(pctColIdx)

	firstDate := dates[0]
	lastDate := dates[len(dates)-1]
	periodLabel := fmt.Sprintf("Periode (%s-%s)",
		indonesianMonthsTitle[firstDate.Month()],
		indonesianMonthsTitle[lastDate.Month()],
	)

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<cols>`)
	b.WriteString(`<col min="1" max="2" width="8.71" customWidth="1"/>`)
	b.WriteString(`<col min="3" max="3" width="37.43" customWidth="1"/>`)
	fmt.Fprintf(&b, `<col min="4" max="%d" width="8.71" customWidth="1"/>`, pctColIdx+1)
	b.WriteString(`</cols>`)
	b.WriteString(`<sheetData>`)

	// Row 1 & Row 2: blank rows above header
	b.WriteString(`<row r="1"/>`)
	b.WriteString(`<row r="2"/>`)

	// Row 3: Main header row
	b.WriteString(`<row r="3">`)
	writeSharedStrCell(&b, "B3", styleManHdrBlue9, ss.add("No"))
	writeSharedStrCell(&b, "C3", styleManHdrBlue9, ss.add("Nama"))
	writeSharedStrCell(&b, "D3", styleManHdrBlue14, ss.add(periodLabel))
	for c := firstDateColIdx + 1; c <= lastDateColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 3), styleManHdrBlue14)
	}
	writeSharedStrCell(&b, cellRef(hkColIdx, 3), styleManHdrBlue9, ss.add("Jumlah HK"))
	writeSharedStrCell(&b, cellRef(hColIdx, 3), styleManHdrBlue9, ss.add("Keterangan"))
	for c := sColIdx; c <= cColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 3), styleManHdrBlue9)
	}
	writeSharedStrCell(&b, cellRef(pctColIdx, 3), styleManHdrBlue9, ss.add("Persentase"))
	b.WriteString(`</row>`)

	// Row 4: Date numbers & Keterangan subheaders (H, S, I, A, L, C)
	b.WriteString(`<row r="4">`)
	writeBlankCell(&b, "B4", styleManHdrBlue9)
	writeBlankCell(&b, "C4", styleManHdrBlue9)
	for i, d := range dates {
		writeNumCell(&b, cellRef(firstDateColIdx+i, 4), styleManDateBlue, strconv.Itoa(excelSerialDate(d)))
	}
	writeBlankCell(&b, cellRef(hkColIdx, 4), styleManHdrBlue9)
	writeSharedStrCell(&b, cellRef(hColIdx, 4), styleManHdrH, ss.add("H"))
	writeSharedStrCell(&b, cellRef(sColIdx, 4), styleManHdrS, ss.add("S"))
	writeSharedStrCell(&b, cellRef(iColIdx, 4), styleManHdrI, ss.add("I"))
	writeSharedStrCell(&b, cellRef(aColIdx, 4), styleManHdrA, ss.add("A"))
	writeSharedStrCell(&b, cellRef(lColIdx, 4), styleManHdrL, ss.add("L"))
	writeSharedStrCell(&b, cellRef(cColIdx, 4), styleManHdrC, ss.add("C"))
	writeBlankCell(&b, cellRef(pctColIdx, 4), styleManHdrBlue9)
	b.WriteString(`</row>`)

	// Row 5 & Row 6: SEKRETARIS WAKIL KETUA UMUM banner (B5:AI6)
	b.WriteString(`<row r="5">`)
	writeSharedStrCell(&b, "B5", styleManBannerBlue, ss.add("SEKRETARIS WAKIL KETUA UMUM"))
	for c := 2; c <= hkColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 5), styleManBannerBlue)
	}
	writeBlankCell(&b, cellRef(hColIdx, 5), styleManHdrH)
	writeBlankCell(&b, cellRef(sColIdx, 5), styleManHdrS)
	writeBlankCell(&b, cellRef(iColIdx, 5), styleManHdrI)
	writeBlankCell(&b, cellRef(aColIdx, 5), styleManHdrA)
	writeBlankCell(&b, cellRef(lColIdx, 5), styleManHdrL)
	writeBlankCell(&b, cellRef(cColIdx, 5), styleManHdrC)
	writeBlankCell(&b, cellRef(pctColIdx, 5), styleManSumHK)
	b.WriteString(`</row>`)

	b.WriteString(`<row r="6">`)
	for c := 1; c <= hkColIdx; c++ {
		writeBlankCell(&b, cellRef(c, 6), styleManBannerBlue)
	}
	writeBlankCell(&b, cellRef(hColIdx, 6), styleManHdrH)
	writeBlankCell(&b, cellRef(sColIdx, 6), styleManHdrS)
	writeBlankCell(&b, cellRef(iColIdx, 6), styleManHdrI)
	writeBlankCell(&b, cellRef(aColIdx, 6), styleManHdrA)
	writeBlankCell(&b, cellRef(lColIdx, 6), styleManHdrL)
	writeBlankCell(&b, cellRef(cColIdx, 6), styleManHdrC)
	writeBlankCell(&b, cellRef(pctColIdx, 6), styleManSumHK)
	b.WriteString(`</row>`)

	merges := []string{
		"B3:B4",
		"C3:C4",
		fmt.Sprintf("D3:%s3", lastDateColName),
		fmt.Sprintf("%s3:%s4", hkColName, hkColName),
		fmt.Sprintf("%s3:%s3", hColName, cColName),
		fmt.Sprintf("%s3:%s4", pctColName, pctColName),
		fmt.Sprintf("B5:%s6", hkColName),
	}

	rowNum := 7
	var dataRanges []string
	for _, g := range groups {
		// Division subheader row
		fmt.Fprintf(&b, `<row r="%d">`, rowNum)
		writeSharedStrCell(&b, cellRef(1, rowNum), styleManDivOrange, ss.add(strings.ToUpper(g.Name)))
		for c := 2; c <= lastDateColIdx; c++ {
			writeBlankCell(&b, cellRef(c, rowNum), styleManDivOrange)
		}
		writeBlankCell(&b, cellRef(hkColIdx, rowNum), styleManSumHK)
		writeBlankCell(&b, cellRef(hColIdx, rowNum), styleManHdrH)
		writeBlankCell(&b, cellRef(sColIdx, rowNum), styleManHdrS)
		writeBlankCell(&b, cellRef(iColIdx, rowNum), styleManHdrI)
		writeBlankCell(&b, cellRef(aColIdx, rowNum), styleManHdrA)
		writeBlankCell(&b, cellRef(lColIdx, rowNum), styleManHdrL)
		writeBlankCell(&b, cellRef(cColIdx, rowNum), styleManHdrC)
		writeBlankCell(&b, cellRef(pctColIdx, rowNum), styleManSumHK)
		b.WriteString(`</row>`)
		merges = append(merges, fmt.Sprintf("B%d:%s%d", rowNum, lastDateColName, rowNum))
		rowNum++

		divFirstEmpRow := rowNum
		for idxInDiv, emp := range g.Employees {
			fmt.Fprintf(&b, `<row r="%d">`, rowNum)
			writeNumCell(&b, cellRef(1, rowNum), styleManNo, strconv.Itoa(idxInDiv+1))
			writeSharedStrCell(&b, cellRef(2, rowNum), styleManName, ss.add(emp.FullName))

			var countH, countS, countI, countA, countL, countC int
			for i, d := range dates {
				code := dailyStatusCode(emp, d, recMap, leavesByEmp[emp.EmployeeID], holidaySet, arrsByEmp[emp.EmployeeID])
				switch code {
				case "H":
					countH++
				case "S":
					countS++
				case "I":
					countI++
				case "A":
					countA++
				case "L":
					countL++
				case "C":
					countC++
				}
				writeSharedStrCell(&b, cellRef(firstDateColIdx+i, rowNum), manualCodeStyle(code), ss.add(code))
			}

			totalHK := countH + countS + countI + countA + countL + countC
			var pct float64
			if totalHK > 0 {
				pct = float64(countH) / float64(totalHK)
			}

			dateRange := fmt.Sprintf("D%d:%s%d", rowNum, lastDateColName, rowNum)
			writeFormulaNumCell(&b, cellRef(hkColIdx, rowNum), styleManSumHK,
				fmt.Sprintf("SUM(%s%d:%s%d)", hColName, rowNum, cColName, rowNum),
				strconv.Itoa(totalHK))
			writeFormulaNumCell(&b, cellRef(hColIdx, rowNum), styleManSumH,
				fmt.Sprintf(`COUNTIF(%s,"H")`, dateRange),
				strconv.Itoa(countH))
			writeFormulaNumCell(&b, cellRef(sColIdx, rowNum), styleManSumS,
				fmt.Sprintf(`COUNTIF(%s,"S")`, dateRange),
				strconv.Itoa(countS))
			writeFormulaNumCell(&b, cellRef(iColIdx, rowNum), styleManSumI,
				fmt.Sprintf(`COUNTIF(%s,"I")`, dateRange),
				strconv.Itoa(countI))
			writeFormulaNumCell(&b, cellRef(aColIdx, rowNum), styleManSumA,
				fmt.Sprintf(`COUNTIF(%s,"A")`, dateRange),
				strconv.Itoa(countA))
			writeFormulaNumCell(&b, cellRef(lColIdx, rowNum), styleManSumL,
				fmt.Sprintf(`COUNTIF(%s,"L")`, dateRange),
				strconv.Itoa(countL))
			writeFormulaNumCell(&b, cellRef(cColIdx, rowNum), styleManSumC,
				fmt.Sprintf(`COUNTIF(%s,"C")`, dateRange),
				strconv.Itoa(countC))
			writeFormulaNumCell(&b, cellRef(pctColIdx, rowNum), styleManSumPct,
				fmt.Sprintf("IF(%s%d=0,0,%s%d/%s%d)", hkColName, rowNum, hColName, rowNum, hkColName, rowNum),
				strconv.FormatFloat(pct, 'f', 10, 64))

			b.WriteString(`</row>`)
			rowNum++
		}
		if rowNum > divFirstEmpRow {
			dataRanges = append(dataRanges, fmt.Sprintf("D%d:%s%d", divFirstEmpRow, lastDateColName, rowNum-1))
		}
	}

	b.WriteString(`</sheetData>`)

	fmt.Fprintf(&b, `<mergeCells count="%d">`, len(merges))
	for _, ref := range merges {
		fmt.Fprintf(&b, `<mergeCell ref="%s"/>`, ref)
	}
	b.WriteString(`</mergeCells>`)

	// Conditional formatting on Sheet 2 (Manual) matching SDM template
	fmt.Fprintf(&b, `<conditionalFormatting sqref="D4:%s4"><cfRule type="expression" dxfId="0" priority="1"><formula>WEEKDAY(D4,2)=7</formula></cfRule></conditionalFormatting>`, lastDateColName)
	if len(dataRanges) > 0 {
		sqref := strings.Join(dataRanges, " ")
		fmt.Fprintf(&b, `<conditionalFormatting sqref="%s">`, sqref)
		b.WriteString(`<cfRule type="cellIs" dxfId="2" priority="2" operator="equal"><formula>"C"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="3" priority="3" operator="equal"><formula>"L"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="4" priority="4" operator="equal"><formula>"A"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="5" priority="5" operator="equal"><formula>"I"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="6" priority="6" operator="equal"><formula>"S"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="9" priority="7" operator="equal"><formula>"T"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="10" priority="8" operator="equal"><formula>"H"</formula></cfRule>`)
		b.WriteString(`<cfRule type="cellIs" dxfId="11" priority="9" operator="equal"><formula>"IS"</formula></cfRule>`)
		b.WriteString(`</conditionalFormatting>`)
	}

	b.WriteString(`</worksheet>`)
	return b.Bytes()
}

// dailyStatusCode menentukan kode presensi harian pada sheet Manual:
// T (belum bergabung / sudah keluar), S (sakit), IS (izin sakit), I (izin), C (cuti),
// A (alpha), WFH (hadir WFA), H (hadir), L (libur shift / hari libur nasional).
func dailyStatusCode(
	emp store.AttendanceExportEmployeesRow,
	d time.Time,
	recMap map[empDayKey]store.AttendanceExportRecordsRow,
	leaves []store.AttendanceExportLeavesRow,
	holidaySet map[string]bool,
	arrs []store.AttendanceExportArrangementsRow,
) string {
	dateStr := d.Format("2006-01-02")
	if emp.JoinDate.Valid {
		jd := emp.JoinDate.Time.UTC().Format("2006-01-02")
		if dateStr < jd {
			return "T"
		}
	}
	if emp.ExitDate.Valid {
		ed := emp.ExitDate.Time.UTC().Format("2006-01-02")
		if dateStr > ed {
			return "T"
		}
	}

	// Check approved leave covering d.
	for _, lv := range leaves {
		if lv.StartDate.Valid && lv.EndDate.Valid {
			sd := lv.StartDate.Time.UTC().Format("2006-01-02")
			ed := lv.EndDate.Time.UTC().Format("2006-01-02")
			if dateStr >= sd && dateStr <= ed {
				return classifyLeaveCode(lv.Code, lv.Name, lv.QuotaKind)
			}
		}
	}

	if rec, ok := recMap[empDayKey{empID: emp.EmployeeID, date: dateStr}]; ok {
		switch rec.Status {
		case store.AttStatusLeave:
			return "C"
		case store.AttStatusAbsent:
			return "A"
		case store.AttStatusPresent, store.AttStatusLate, store.AttStatusReview:
			if strings.EqualFold(rec.WorkMode, "wfa") {
				return "WFH"
			}
			return "H"
		}
	}

	if holidaySet[dateStr] || !isShiftWorkDay(emp.WorkDays, d.Weekday()) {
		return "L"
	}

	for _, arr := range arrs {
		if !arr.StartsOn.Valid {
			continue
		}
		sd := arr.StartsOn.Time.UTC().Format("2006-01-02")
		if dateStr < sd {
			continue
		}
		if arr.EndsOn.Valid && dateStr > arr.EndsOn.Time.UTC().Format("2006-01-02") {
			continue
		}
		if len(arr.Weekdays) == 0 || isShiftWorkDay(arr.Weekdays, d.Weekday()) {
			if strings.EqualFold(arr.Mode, "wfa") {
				return "WFH"
			}
		}
	}

	return "H"
}

func classifyLeaveCode(code, name string, quotaKind store.LeaveQuotaKind) string {
	cu := strings.ToUpper(strings.TrimSpace(code))
	nu := strings.ToUpper(strings.TrimSpace(name))
	if cu == "IS" || cu == "IZIN_SAKIT" || strings.Contains(nu, "IZIN SAKIT") {
		return "IS"
	}
	if cu == "S" || cu == "SAKIT" || strings.Contains(nu, "SAKIT") {
		return "S"
	}
	if cu == "I" || cu == "IZIN" || strings.Contains(nu, "IZIN") || quotaKind == store.LeaveQuotaKindNone {
		return "I"
	}
	return "C"
}

func isShiftWorkDay(workDays []int16, wd time.Weekday) bool {
	if len(workDays) == 0 {
		return wd != time.Sunday
	}
	isoDay := int16(wd)
	if wd == time.Sunday {
		isoDay = 7
	}
	for _, d := range workDays {
		if d == isoDay {
			return true
		}
	}
	return false
}

func manualCodeStyle(code string) int {
	switch code {
	case "L":
		return styleManCodeL
	case "S":
		return styleManCodeS
	case "I":
		return styleManCodeI
	case "A":
		return styleManCodeA
	case "C":
		return styleManCodeC
	case "IS":
		return styleManCodeIS
	case "T":
		return styleManCodeT
	default:
		return styleManCodeH
	}
}

// excelSerialDate mengubah tanggal UTC menjadi nomor seri tanggal Excel (sistem 1900).
func excelSerialDate(t time.Time) int {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	utcDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return int(utcDay.Sub(epoch).Hours() / 24)
}

// excelTimeFraction mengubah waktu menjadi pecahan hari (0.0 .. 1.0) dalam zona waktu WIB.
func excelTimeFraction(t time.Time) string {
	w := t.In(pgconv.WIB)
	secs := w.Hour()*3600 + w.Minute()*60 + w.Second()
	frac := float64(secs) / 86400.0
	return strconv.FormatFloat(frac, 'f', 16, 64)
}

// excelColName mengubah indeks kolom 0-based menjadi huruf kolom Excel (0 -> A, 25 -> Z, 26 -> AA).
func excelColName(col int) string {
	var buf [4]byte
	i := len(buf)
	for col >= 0 {
		i--
		buf[i] = byte('A' + (col % 26))
		col = col/26 - 1
	}
	return string(buf[i:])
}

func cellRef(colZeroBased, rowOneBased int) string {
	return excelColName(colZeroBased) + strconv.Itoa(rowOneBased)
}

func writeSharedStrCell(b *bytes.Buffer, ref string, styleIdx, strIdx int) {
	fmt.Fprintf(b, `<c r="%s" s="%d" t="s"><v>%d</v></c>`, ref, styleIdx, strIdx)
}

func writeNumCell(b *bytes.Buffer, ref string, styleIdx int, numVal string) {
	fmt.Fprintf(b, `<c r="%s" s="%d"><v>%s</v></c>`, ref, styleIdx, numVal)
}

func writeFormulaNumCell(b *bytes.Buffer, ref string, styleIdx int, formula, cachedVal string) {
	fmt.Fprintf(b, `<c r="%s" s="%d"><f>%s</f><v>%s</v></c>`, ref, styleIdx, xmlEscape(formula), cachedVal)
}

func writeBlankCell(b *bytes.Buffer, ref string, styleIdx int) {
	fmt.Fprintf(b, `<c r="%s" s="%d"/>`, ref, styleIdx)
}

var xmlTextReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func xmlEscape(s string) string {
	return xmlTextReplacer.Replace(s)
}

type sharedStrings struct {
	items []string
	index map[string]int
}

func newSharedStrings() *sharedStrings {
	return &sharedStrings{index: make(map[string]int)}
}

func (s *sharedStrings) add(val string) int {
	if idx, ok := s.index[val]; ok {
		return idx
	}
	idx := len(s.items)
	s.items = append(s.items, val)
	s.index[val] = idx
	return idx
}

func (s *sharedStrings) marshal() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&b, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, len(s.items), len(s.items))
	for _, it := range s.items {
		if strings.TrimSpace(it) != it {
			fmt.Fprintf(&b, `<si><t xml:space="preserve">%s</t></si>`, xmlEscape(it))
		} else {
			fmt.Fprintf(&b, `<si><t>%s</t></si>`, xmlEscape(it))
		}
	}
	b.WriteString(`</sst>`)
	return b.Bytes()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
</Relationships>`

const corePropsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:creator>HRIS As-Syifa</dc:creator>
  <dc:title>Rekap Absen Yayasan</dc:title>
</cp:coreProperties>`

const workbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <workbookPr/>
  <sheets>
    <sheet state="visible" name="Fingerprint" sheetId="1" r:id="rId1"/>
    <sheet state="visible" name="Manual" sheetId="2" r:id="rId2"/>
  </sheets>
  <calcPr fullCalcOnLoad="1"/>
</workbook>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <numFmts count="1">
    <numFmt numFmtId="164" formatCode="d"/>
  </numFmts>
  <fonts count="10">
    <font><sz val="10"/><name val="Calibri"/></font>
    <font><b/><sz val="10"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
    <font><b/><sz val="10"/><color rgb="FF000000"/><name val="Calibri"/></font>
    <font><b/><sz val="9"/><color rgb="FFFFFFFF"/><name val="Arial"/></font>
    <font><b/><sz val="14"/><color rgb="FFFFFFFF"/><name val="Arial"/></font>
    <font><b/><sz val="9"/><color rgb="FF000000"/><name val="Arial"/></font>
    <font><b/><sz val="16"/><color rgb="FFFFFFFF"/><name val="Arial"/></font>
    <font><b/><sz val="16"/><color rgb="FF000000"/><name val="Arial"/></font>
    <font><b/><sz val="12"/><color rgb="FF000000"/><name val="Calibri"/></font>
    <font><sz val="9"/><color rgb="FF000000"/><name val="Arial"/></font>
  </fonts>
  <fills count="20">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF1B5E20"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFDADADA"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0D47A1"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFECECEC"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFF2F2F2"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFC7CE"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFC8C8C8"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFF7CAAC"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFC5E0B3"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFF0000"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFBDD6EE"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFF4B083"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFE2EFD9"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFB4C6E7"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF92D050"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0070C0"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFE3F2FD"/><bgColor indexed="64"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color auto="1"/></left>
      <right style="thin"><color auto="1"/></right>
      <top style="thin"><color auto="1"/></top>
      <bottom style="thin"><color auto="1"/></bottom>
      <diagonal/>
    </border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="41">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf>
    <xf numFmtId="17" fontId="2" fillId="3" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="164" fontId="1" fillId="4" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="2" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="20" fontId="0" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="20" fontId="0" fillId="7" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="3" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf>
    <xf numFmtId="0" fontId="4" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="164" fontId="3" fillId="4" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="8" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="9" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="10" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="11" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="12" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="13" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="6" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="7" fillId="14" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="8" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="left" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="15" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="16" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="9" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="10" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="11" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="13" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="17" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="18" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="19" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="8" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="9" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="10" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="11" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="12" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="9" fillId="13" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="9" fontId="9" fillId="19" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
  </cellXfs>
  <dxfs count="13">
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFED7D31"/><bgColor rgb="FFED7D31"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFBFBFBF"/><bgColor rgb="FFBFBFBF"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor rgb="FFFFFF00"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFB4C6E7"/><bgColor rgb="FFB4C6E7"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFFF0000"/><bgColor rgb="FFFF0000"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFC5E0B3"/><bgColor rgb="FFC5E0B3"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFF7CAAC"/><bgColor rgb="FFF7CAAC"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FF548135"/><bgColor rgb="FF548135"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FF8496B0"/><bgColor rgb="FF8496B0"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FF0070C0"/><bgColor rgb="FF0070C0"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFC8C8C8"/><bgColor rgb="FFC8C8C8"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FF92D050"/><bgColor rgb="FF92D050"/></patternFill></fill><border/></dxf>
    <dxf><font/><fill><patternFill patternType="solid"><fgColor rgb="FFF4CCCC"/><bgColor rgb="FFF4CCCC"/></patternFill></fill><border/></dxf>
  </dxfs>
</styleSheet>`
