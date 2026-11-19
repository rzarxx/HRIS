package announcement

import "testing"

func TestDetectAttachment(t *testing.T) {
	cases := []struct {
		data []byte
		want string
		ok   bool
	}{
		{[]byte("%PDF-1.7 isi"), "application/pdf", true},
		{[]byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg", true},
		{[]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n', 0}, "image/png", true},
		{[]byte("<html>bukan berkas</html>"), "", false},
		{[]byte("PK\x03\x04 zip"), "", false},
	}
	for _, c := range cases {
		got, _, ok := detectAttachment(c.data)
		if got != c.want || ok != c.ok {
			t.Errorf("detectAttachment(%q) = %q %v, ingin %q %v", c.data[:4], got, ok, c.want, c.ok)
		}
	}
}

func TestCleanFileName(t *testing.T) {
	cases := map[string]string{
		"Surat Edaran.pdf":         "Surat Edaran.pdf",
		"../../etc/passwd":         "passwd.pdf",
		`C:\Users\sdm\jadwal.PDF`:  "jadwal.pdf",
		"foto.png":                 "foto.pdf",
		"  ":                       "lampiran.pdf",
		"nama\x00aneh\"berkas.pdf": "namaanehberkas.pdf",
	}
	for in, want := range cases {
		if got := cleanFileName(in, ".pdf"); got != want {
			t.Errorf("cleanFileName(%q) = %q, ingin %q", in, got, want)
		}
	}
}
