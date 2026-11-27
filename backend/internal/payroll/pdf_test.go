package payroll

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestTerbilang(t *testing.T) {
	cases := map[int64]string{
		0:          "nol",
		11:         "sebelas",
		15:         "lima belas",
		21:         "dua puluh satu",
		100:        "seratus",
		115:        "seratus lima belas",
		1000:       "seribu",
		1500:       "seribu lima ratus",
		25000:      "dua puluh lima ribu",
		1_000_000:  "satu juta",
		4973250:    "empat juta sembilan ratus tujuh puluh tiga ribu dua ratus lima puluh",
		2000000000: "dua miliar",
	}
	for n, want := range cases {
		if got := Terbilang(n); got != want {
			t.Errorf("Terbilang(%d) = %q, ingin %q", n, got, want)
		}
	}
}

func TestRupiah(t *testing.T) {
	for in, want := range map[string]string{"0": "Rp 0", "999": "Rp 999", "1000": "Rp 1.000", "4973250.4": "Rp 4.973.250", "-1500": "-Rp 1.500"} {
		if got := rupiah(decimal.RequireFromString(in)); got != want {
			t.Errorf("rupiah(%s) = %q, ingin %q", in, got, want)
		}
	}
}

func TestRenderPDFDeterministic(t *testing.T) {
	qty, rate := "4.00", "20000.00"
	doc := SlipDoc{
		ID: "8f0c2a1e-1111-2222-3333-444455556666", Year: 2026, Month: 11,
		Start: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC),
		Status: StatusFinal, IssuedAt: time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC),
		Nip: "PJ2019", FullName: "Agus Permana", Position: "Petugas Gudang", Division: "Operasional", Subdivision: "Gudang",
		Items: []ItemView{
			{Code: BaseCode, Name: "Ubar pokok", Kind: KindEarning, Amount: "4600000.00", ShowOnSlip: true},
			{Code: "LEMBUR", Name: "Lembur", Kind: KindEarning, Quantity: &qty, Rate: &rate, Amount: "80000.00", ShowOnSlip: true},
			{Code: "RAHASIA", Name: "Tidak tampil", Kind: KindEarning, Amount: "1.00", ShowOnSlip: false},
			{Code: "PPH21", Name: "PPh 21", Kind: KindDeduction, Amount: "117000.00", ShowOnSlip: true},
		},
		Gross: decimal.RequireFromString("4680001"), Deductions: decimal.RequireFromString("117000"), Net: decimal.RequireFromString("4563001"),
	}
	a, err := RenderPDF(doc)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderPDF(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(a, []byte("%PDF-")) || !bytes.Equal(a, b) {
		t.Fatalf("PDF tidak valid atau tidak deterministik (%d vs %d byte)", len(a), len(b))
	}
	if DocumentNumber(doc) != "SU/2026/11/8F0C2A1E" || PDFFileName(doc) != "Slip-Ubar-2026-11-PJ2019.pdf" {
		t.Errorf("nomor %s, berkas %s", DocumentNumber(doc), PDFFileName(doc))
	}
}
