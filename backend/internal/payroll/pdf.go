package payroll

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

//go:embed assets/logo.png
var logoPNG []byte

// OrgName tampil di kop slip.
const OrgName = "Yayasan Al Bahjah"

var monthNames = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// MonthName mengembalikan nama bulan Indonesia.
func MonthName(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return monthNames[m]
}

// SlipDoc adalah isi satu slip ubar untuk PDF; seluruhnya dari salinan slip yang tidak berubah setelah final.
type SlipDoc struct {
	ID          string
	Year, Month int
	Start, End  time.Time
	Status      string // status periode; selain final/paid slip diberi tanda DRAF
	IssuedAt    time.Time
	Nip         string
	FullName    string
	Position    string
	Division    string
	Subdivision string
	Attendance  AttendanceSummary
	Items       []ItemView
	Gross       decimal.Decimal
	Deductions  decimal.Decimal
	Net         decimal.Decimal
}

// rupiah memformat nominal tanpa sen dengan titik ribuan: 1.234.500.
func rupiah(d decimal.Decimal) string {
	neg := d.IsNegative()
	s := d.Abs().Round(0).String()
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp " + b.String()
	}
	return "Rp " + b.String()
}

func dateID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), MonthName(int(t.Month())), t.Year())
}

var ones = [...]string{"", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas"}

// Terbilang mengubah bilangan bulat non-negatif menjadi kata dalam bahasa Indonesia.
func Terbilang(n int64) string {
	if n == 0 {
		return "nol"
	}
	return strings.TrimSpace(terbilang(n))
}

func terbilang(n int64) string {
	switch {
	case n < 12:
		return ones[n]
	case n < 20:
		return terbilang(n-10) + " belas"
	case n < 100:
		return strings.TrimSpace(terbilang(n/10) + " puluh " + terbilang(n%10))
	case n < 200:
		return strings.TrimSpace("seratus " + terbilang(n-100))
	case n < 1000:
		return strings.TrimSpace(terbilang(n/100) + " ratus " + terbilang(n%100))
	case n < 2000:
		return strings.TrimSpace("seribu " + terbilang(n-1000))
	case n < 1_000_000:
		return strings.TrimSpace(terbilang(n/1000) + " ribu " + terbilang(n%1000))
	case n < 1_000_000_000:
		return strings.TrimSpace(terbilang(n/1_000_000) + " juta " + terbilang(n%1_000_000))
	case n < 1_000_000_000_000:
		return strings.TrimSpace(terbilang(n/1_000_000_000) + " miliar " + terbilang(n%1_000_000_000))
	default:
		return strings.TrimSpace(terbilang(n/1_000_000_000_000) + " triliun " + terbilang(n%1_000_000_000_000))
	}
}

// DocumentNumber adalah nomor slip yang stabil: SU/2026/11/<8 karakter awal ID slip>.
func DocumentNumber(d SlipDoc) string {
	short := strings.ToUpper(strings.ReplaceAll(d.ID, "-", ""))
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("SU/%d/%02d/%s", d.Year, d.Month, short)
}

// PDFFileName adalah nama berkas unduhan slip.
func PDFFileName(d SlipDoc) string {
	return fmt.Sprintf("Slip-Ubar-%d-%02d-%s.pdf", d.Year, d.Month, d.Nip)
}

// RenderPDF menyusun slip ubar A4 potret agar mudah dicetak. Hasilnya deterministik untuk
// isi yang sama (tanggal pembuatan PDF memakai tanggal terbit).
func RenderPDF(d SlipDoc) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(d.IssuedAt)
	pdf.SetModificationDate(d.IssuedAt)
	pdf.SetTitle(fmt.Sprintf("Slip Ubar %s %d - %s", MonthName(d.Month), d.Year, d.FullName), true)
	pdf.SetAuthor(OrgName, true)
	pdf.SetMargins(18, 16, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	w, _ := pdf.GetPageSize()
	inner := w - 36

	// Kop.
	pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(logoPNG))
	pdf.ImageOptions("logo", 18, 14, 30, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	pdf.SetXY(52, 16)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(inner-34, 7, tr(OrgName), "", 2, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(inner-34, 6, tr(fmt.Sprintf("Slip Ubar %s %d", MonthName(d.Month), d.Year)), "", 2, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(inner-34, 5, tr("No. "+DocumentNumber(d)), "", 2, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(38)
	pdf.SetDrawColor(21, 128, 61)
	pdf.SetLineWidth(0.6)
	pdf.Line(18, pdf.GetY(), w-18, pdf.GetY())
	pdf.Ln(4)

	if d.Status != StatusFinal && d.Status != StatusPaid {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(180, 83, 9)
		pdf.CellFormat(inner, 6, tr("DRAF - belum disetujui, bukan slip resmi"), "", 1, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(1)
	}

	// Identitas dan periode.
	info := [][2]string{
		{"Nama", d.FullName},
		{"NIPY", d.Nip},
		{"Jabatan", d.Position},
		{"Subdiv / Divisi", d.Subdivision + " / " + d.Division},
		{"Periode presensi", dateID(d.Start) + " sampai " + dateID(d.End)},
	}
	pdf.SetFont("Helvetica", "", 10)
	for _, row := range info {
		if row[1] == "" {
			continue
		}
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(42, 6, tr(row[0]), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(inner-42, 6, tr(row[1]), "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)

	// Ringkasan presensi.
	a := d.Attendance
	section := func(title string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetFillColor(240, 253, 244)
		pdf.CellFormat(inner, 7, tr(title), "", 1, "L", true, 0, "")
		pdf.Ln(1)
	}
	section("Ringkasan presensi")
	pdf.SetFont("Helvetica", "", 10)
	stats := [][2]string{
		{"Hari kerja", a.WorkDays.String()},
		{"Hadir", fmt.Sprint(a.PresentDays)},
		{"Telat", fmt.Sprintf("%d kali (%d menit)", a.LateCount, a.LateMinutes)},
		{"Alpha", fmt.Sprint(a.AbsentDays)},
		{"Cuti/izin", fmt.Sprint(a.LeaveDays)},
	}
	col := inner / 5
	for _, s := range stats {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(col, 5, tr(s[0]), "", 0, "L", false, 0, "")
	}
	pdf.Ln(5)
	pdf.SetTextColor(0, 0, 0)
	for _, s := range stats {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(col, 6, tr(s[1]), "", 0, "L", false, 0, "")
	}
	pdf.Ln(9)

	itemRows := func(kind string) {
		for _, it := range d.Items {
			if it.Kind != kind || !it.ShowOnSlip {
				continue
			}
			label := it.Name
			if it.Quantity != nil && it.Rate != nil {
				q, _ := decimal.NewFromString(*it.Quantity)
				r, _ := decimal.NewFromString(*it.Rate)
				label += fmt.Sprintf(" (%s x %s)", q.String(), rupiah(r))
			}
			amount, _ := decimal.NewFromString(it.Amount)
			pdf.SetFont("Helvetica", "", 10)
			pdf.CellFormat(inner-45, 6.5, tr(label), "B", 0, "L", false, 0, "")
			pdf.CellFormat(45, 6.5, tr(rupiah(amount)), "B", 1, "R", false, 0, "")
		}
	}
	total := func(label string, v decimal.Decimal) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(inner-45, 7, tr(label), "", 0, "L", false, 0, "")
		pdf.CellFormat(45, 7, tr(rupiah(v)), "", 1, "R", false, 0, "")
		pdf.Ln(3)
	}
	pdf.SetDrawColor(226, 232, 240)
	pdf.SetLineWidth(0.2)
	section("Pendapatan")
	itemRows(KindEarning)
	total("Total pendapatan", d.Gross)
	section("Potongan")
	itemRows(KindDeduction)
	total("Total potongan", d.Deductions)

	// Ubar bersih.
	pdf.SetFillColor(21, 128, 61)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(inner-60, 10, tr("  Ubar bersih"), "", 0, "L", true, 0, "")
	pdf.CellFormat(60, 10, tr(rupiah(d.Net)+"  "), "", 1, "R", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "I", 9)
	words := Terbilang(d.Net.Round(0).IntPart())
	pdf.MultiCell(inner, 5.5, tr("Terbilang: "+strings.ToUpper(words[:1])+words[1:]+" rupiah"), "", "L", false)
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(100, 100, 100)
	issued := d.IssuedAt.In(pgconv.WIB)
	pdf.MultiCell(inner, 4.5, tr(fmt.Sprintf(
		"Diterbitkan %s pukul %s WIB melalui HRIS %s. Slip ini dibuat sistem dan sah tanpa tanda tangan. "+
			"Rahasiakan isi slip ini; hubungi SDM bila ada yang tidak sesuai.",
		dateID(issued), issued.Format("15.04"), OrgName)), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
