package fieldcrypt

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c, err := New(bytes.Repeat([]byte{7}, keySize))
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Encrypt([]byte("rahasia"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.Encrypt([]byte("rahasia"))
	if bytes.Equal(a, b) {
		t.Error("nonce harus membuat ciphertext berbeda")
	}
	got, err := c.Decrypt(a)
	if err != nil || string(got) != "rahasia" {
		t.Errorf("dekripsi: %q, %v", got, err)
	}
}

func TestRejectsTamperingAndWrongKey(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{7}, keySize))
	other, _ := New(bytes.Repeat([]byte{8}, keySize))
	data, _ := c.Encrypt([]byte("rahasia"))
	if _, err := other.Decrypt(data); err == nil {
		t.Error("kunci lain tidak boleh berhasil")
	}
	data[len(data)-1] ^= 1
	if _, err := c.Decrypt(data); err == nil {
		t.Error("data yang diubah harus ditolak")
	}
	if _, err := c.Decrypt([]byte("x")); err == nil {
		t.Error("data pendek harus ditolak")
	}
	if _, err := New([]byte("pendek")); err == nil {
		t.Error("kunci salah ukuran harus ditolak")
	}
}

func TestLoadOrCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "key")
	if _, err := LoadOrCreate(path, false); err == nil {
		t.Error("tanpa izin membuat harus error")
	}
	first, err := LoadOrCreate(path, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := first.Encrypt([]byte("x"))
	second, err := LoadOrCreate(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := second.Decrypt(data); err != nil || string(got) != "x" {
		t.Errorf("kunci yang dibaca ulang berbeda: %v", err)
	}
}
