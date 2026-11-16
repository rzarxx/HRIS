package auth

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
)

// InitialPassword membuat kata sandi awal "@" + nama depan + tahun pembuatan (WIB), misalnya
// @Siti2026 (D53). Polanya bisa ditebak, jadi akun selalu dibuat dengan must_change_password
// dan dilindungi lockout; kata sandi pilihan pengguna tetap mengikuti ValidatePassword.
// InitialPasswordTTL membatasi berlakunya kata sandi awal. Lewat dari itu login ditolak
// AUTH_INITIAL_PASSWORD_EXPIRED dan SDM perlu mereset ulang (D53).
const InitialPasswordTTL = 72 * time.Hour

func InitialPassword(fullName string, now time.Time) string {
	return "@" + firstName(fullName) + strconv.Itoa(now.In(pgconv.WIB).Year())
}

// firstName mengambil kata pertama yang memuat minimal dua huruf, tanpa tanda baca, dengan huruf
// pertama kapital. "M. Rizal" menjadi "Rizal"; nama tanpa kata seperti itu menjadi "Pejuang".
func firstName(fullName string) string {
	for _, word := range strings.Fields(fullName) {
		letters := make([]rune, 0, len(word))
		for _, r := range word {
			if unicode.IsLetter(r) {
				letters = append(letters, unicode.ToLower(r))
			}
		}
		if len(letters) >= 2 {
			letters[0] = unicode.ToUpper(letters[0])
			return string(letters)
		}
	}
	return "Pejuang"
}
