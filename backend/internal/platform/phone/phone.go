// Package phone menormalkan nomor WA Indonesia ke satu bentuk baku (628…), yang dipakai untuk
// disimpan, dicari, dan login (D53).
package phone

import "strings"

// Normalize menerima 08…, 628…, +62 8…, atau 8… dengan spasi, titik, atau tanda minus, lalu
// mengembalikan digit berawalan 628. ok bernilai false bila bukan nomor HP Indonesia yang valid.
func Normalize(raw string) (string, bool) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0, r == ' ', r == '-', r == '.', r == '(', r == ')':
		default:
			return "", false
		}
	}
	digits := b.String()
	switch {
	case strings.HasPrefix(digits, "62"):
	case strings.HasPrefix(digits, "0"):
		digits = "62" + digits[1:]
	case strings.HasPrefix(digits, "8"):
		digits = "62" + digits
	default:
		return "", false
	}
	// 628 diikuti 7 sampai 12 digit, sama dengan CHECK chk_employees_phone.
	if !strings.HasPrefix(digits, "628") || len(digits) < 10 || len(digits) > 15 {
		return "", false
	}
	return digits, true
}
