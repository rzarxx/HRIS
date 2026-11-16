package phone

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"081234567890":      "6281234567890",
		"+62 812-3456-7890": "6281234567890",
		"6281234567890":     "6281234567890",
		"81234567890":       "6281234567890",
		"0812.3456.789":     "628123456789",
		" (0812) 3456789 ":  "628123456789",
	}
	for in, want := range cases {
		got, ok := Normalize(in)
		if !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; mau %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "0221234567", "6221234567", "0812", "08123456789012345", "0812a3456789", "62-+812345678"} {
		if got, ok := Normalize(bad); ok {
			t.Errorf("Normalize(%q) = %q, mau ditolak", bad, got)
		}
	}
}
