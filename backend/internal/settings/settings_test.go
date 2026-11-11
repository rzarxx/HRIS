package settings

import (
	"testing"
	"time"
)

func TestRenderFooterUsesWIBYear(t *testing.T) {
	// 31 Desember 2026 pukul 18.00 UTC sudah 1 Januari 2027 di WIB.
	at := time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC)
	if got := RenderFooter(DefaultFooter, at); got != "© 2027 PUTI (Pusat Teknologi Informasi) Al Bahjah" {
		t.Errorf("footer = %q", got)
	}
	if got := RenderFooter("Hak cipta {tahun}–{tahun}", at); got != "Hak cipta 2027–2027" {
		t.Errorf("semua token diganti: %q", got)
	}
	if got := RenderFooter("Tanpa tahun", at); got != "Tanpa tahun" {
		t.Errorf("tanpa token: %q", got)
	}
}

func TestCleanFooterIsOneLinePlainText(t *testing.T) {
	if got := cleanFooter("  PUTI\nAl\tBahjah \x07 {tahun}  "); got != "PUTI Al Bahjah {tahun}" {
		t.Errorf("bersih = %q", got)
	}
}
