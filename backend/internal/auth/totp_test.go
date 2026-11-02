package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// Vektor uji RFC 6238 lampiran B (SHA1, kunci ASCII "12345678901234567890"), 6 digit terakhir.
func TestTOTPVectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	cases := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		step, ok := VerifyTOTP(secret, c.code, time.Unix(c.unix, 0), 0)
		if !ok {
			t.Errorf("kode %s pada %d ditolak", c.code, c.unix)
		}
		if ok && step != c.unix/totpPeriod {
			t.Errorf("langkah = %d", step)
		}
	}
}

func TestTOTPRejects(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	at := time.Unix(1111111111, 0)
	if _, ok := VerifyTOTP(secret, "000000", at, 0); ok {
		t.Error("kode salah diterima")
	}
	if _, ok := VerifyTOTP(secret, "12345", at, 0); ok {
		t.Error("panjang salah diterima")
	}
	if _, ok := VerifyTOTP("bukan base32!!", "050471", at, 0); ok {
		t.Error("secret rusak diterima")
	}
	// Kode yang sudah dipakai tidak boleh diterima lagi.
	step, ok := VerifyTOTP(secret, "050471", at, 0)
	if !ok {
		t.Fatal("kode valid ditolak")
	}
	if _, ok := VerifyTOTP(secret, "050471", at, step); ok {
		t.Error("kode dipakai ulang diterima")
	}
	// Di luar jendela ±30 detik ditolak.
	if _, ok := VerifyTOTP(secret, "050471", at.Add(5*time.Minute), 0); ok {
		t.Error("kode kedaluwarsa diterima")
	}
}

func TestTOTPURL(t *testing.T) {
	u := TOTPURL("ABC", "sdm@pejuang.id")
	for _, want := range []string{"otpauth://totp/", "secret=ABC", "issuer=HRIS", "period=30", "digits=6"} {
		if !strings.Contains(u, want) {
			t.Errorf("%s tidak memuat %s", u, want)
		}
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := NewRecoveryCodes(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("jumlah kode %d/%d", len(codes), len(hashes))
	}
	if codes[0] == hashes[0] {
		t.Error("yang disimpan harus hash, bukan kode asli")
	}
	rest, ok := ConsumeRecoveryCode(hashes, strings.ToUpper(codes[3]))
	if !ok || len(rest) != 7 {
		t.Fatalf("pakai kode: ok=%v sisa=%d", ok, len(rest))
	}
	if _, ok := ConsumeRecoveryCode(rest, codes[3]); ok {
		t.Error("kode pemulihan dipakai dua kali")
	}
	if _, ok := ConsumeRecoveryCode(hashes, "salah"); ok {
		t.Error("kode salah diterima")
	}
}
