package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

// HeaderAppVersion dikirim aplikasi Android: "versionName+versionCode", mis. "0.14.0+1400" (MASTER.md §7.2).
const HeaderAppVersion = "X-App-Version"

// appVersionOpen tetap dilayani untuk aplikasi lama, agar aplikasi bisa mengetahui versi terbaru, menampilkan
// layar perbarui, dan tetap bisa masuk atau keluar (D63).
var appVersionOpen = map[string]bool{
	"/v1/app/version":         true,
	"/v1/branding":            true,
	"/v1/auth/mobile/login":   true,
	"/v1/auth/mobile/refresh": true,
	"/v1/auth/logout":         true,
}

// parseAppVersion membaca versionCode dari header. Header tanpa "+" boleh berisi kode saja.
func parseAppVersion(header string) (int, bool) {
	header = strings.TrimSpace(header)
	if i := strings.LastIndex(header, "+"); i >= 0 {
		header = header[i+1:]
	}
	code, err := strconv.Atoi(header)
	if err != nil || code <= 0 {
		return 0, false
	}
	return code, true
}

// requireAppVersion menolak aplikasi di bawah versi minimum dengan 426 APP_UPDATE_REQUIRED. Hanya berlaku
// bila header X-App-Version ada, jadi web admin tidak terpengaruh.
func requireAppVersion(minCode func(context.Context) int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if minCode == nil || appVersionOpen[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			code, ok := parseAppVersion(r.Header.Get(HeaderAppVersion))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if min := minCode(r.Context()); min > 0 && code < min {
				err := httpx.NewError(http.StatusUpgradeRequired, "APP_UPDATE_REQUIRED", "Versi aplikasi ini sudah tidak didukung. Silakan perbarui aplikasi.")
				err.Details = map[string]any{"min_code": min}
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
