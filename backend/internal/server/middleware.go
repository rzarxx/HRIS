package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

const (
	maxBodyBytes   = 1 << 20 // 1 MB, MASTER.md §5.7
	maxUploadBytes = 5 << 20 // 5 MB untuk route yang menerima selfie
	requestTimeout = 15 * time.Second

	// Batas per akun 120/menit (MASTER.md §5.7). Batas per IP dibuat longgar karena satu
	// kantor bisa berbagi satu IP publik; fungsinya menahan banjir dari klien tak dikenal.
	clockLimitPerMinute = 10
	userLimitPerMinute  = 120
	ipLimitPerMinute    = 1200
)

// Options adalah konfigurasi middleware yang berbeda antar lingkungan.
type Options struct {
	AllowedOrigins []string
	Secure         bool
	TrustProxy     bool
}

// realIP mengganti alamat klien dengan X-Real-IP dari reverse proxy. Header hanya dipercaya
// bila koneksi datang dari mesin yang sama, sehingga klien luar tidak bisa memalsukannya.
// Nginx wajib menimpa header ini: proxy_set_header X-Real-IP $remote_addr.
func realIP(trust bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trust {
				if remote := audit.ClientIP(r); remote != nil && remote.IsLoopback() {
					if addr, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
						r.RemoteAddr = netip.AddrPortFrom(addr.Unmap(), 0).String()
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// blockDirectAccess menolak request yang bukan dari web admin atau aplikasi (D37). Header bisa
// dipalsukan, jadi ini hanya menutup akses langsung dan pemindaian biasa. Perlindungan data tetap
// berada di autentikasi dan permission.
func blockDirectAccess(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slices.Contains(allowed, r.Header.Get("Origin")) || r.Header.Get("Authorization") != "" || isDirectLocal(r) || isMobileAuth(r) {
				next.ServeHTTP(w, r)
				return
			}
			if len(allowed) > 0 && isBrowserNavigation(r) {
				// Tujuannya dari konfigurasi, bukan dari request, jadi tidak bisa dipakai untuk open redirect.
				http.Redirect(w, r, allowed[0]+"/", http.StatusFound)
				return
			}
			httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "API_ACCESS_DENIED", "Akses langsung ke API tidak diizinkan."))
		})
	}
}

// isDirectLocal menjawab apakah koneksi datang langsung dari mesin ini, misalnya pemeriksaan
// deploy. Nginx selalu menimpa X-Real-IP, jadi request dari luar tidak bisa lolos sebagai lokal.
func isDirectLocal(r *http.Request) bool {
	if r.Header.Get("X-Real-IP") != "" {
		return false
	}
	addr := audit.ClientIP(r)
	return addr != nil && addr.IsLoopback()
}

// isMobileAuth membuka login dan refresh mobile, karena aplikasi belum punya token saat memanggilnya
// dan tidak mengirim Origin. Keduanya tetap wajib kredensial (D37).
func isMobileAuth(r *http.Request) bool {
	return r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/auth/mobile/")
}

func isBrowserNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.Header.Get("Sec-Fetch-Mode") == "navigate" || strings.Contains(r.Header.Get("Accept"), "text/html")
}

// limitByIP berjalan sebelum autentikasi (urutan F2).
func limitByIP(limiter auth.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "ip:"
			if addr := audit.ClientIP(r); addr != nil {
				key += addr.String()
			}
			if ok, wait := limiter.Allow(key, ipLimitPerMinute, time.Minute, time.Now()); !ok {
				httpx.WriteError(w, r, auth.RateLimited(w, wait))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitByUser berjalan setelah autentikasi; kuncinya user yang sudah terverifikasi.
func limitByUser(limiter auth.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			if ok, wait := limiter.Allow("user:"+p.UserID.String(), userLimitPerMinute, time.Minute, time.Now()); !ok {
				httpx.WriteError(w, r, auth.RateLimited(w, wait))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clockPaths adalah route presensi dengan batas lebih ketat (MASTER.md §5.7).
var clockPaths = map[string]bool{
	"/v1/attendance/challenge": true,
	"/v1/attendance/clock-in":  true,
	"/v1/attendance/clock-out": true,
}

// limitClock membatasi presensi per akun dan IP.
func limitClock(limiter auth.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthenticated())
				return
			}
			key := "clock:" + p.UserID.String() + "|"
			if addr := audit.ClientIP(r); addr != nil {
				key += addr.String()
			}
			if ok, wait := limiter.Allow(key, clockLimitPerMinute, time.Minute, time.Now()); !ok {
				httpx.WriteError(w, r, auth.RateLimited(w, wait))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(httpx.WithRequestID(r.Context(), id)))
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.ErrorContext(r.Context(), "panic", "request_id", httpx.RequestID(r.Context()), "panic", rec)
				httpx.WriteError(w, r, httpx.NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server."))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// requestLogger mencatat pola route, bukan URL mentah, agar query dan ID tidak masuk log.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := ""
		if rc := chi.RouteContext(r.Context()); rc != nil {
			route = rc.RoutePattern()
		}
		slog.InfoContext(r.Context(), "request",
			"request_id", httpx.RequestID(r.Context()),
			"method", r.Method, "route", route, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// uploadRoutes menerima selfie, jadi batas body-nya lebih besar (MASTER.md §5.7).
var uploadRoutes = map[string]bool{
	"/v1/attendance/clock-in":  true,
	"/v1/attendance/clock-out": true,
	"/v1/face/enroll":          true,
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(maxBodyBytes)
		if r.Method == http.MethodPost && uploadRoutes[r.URL.Path] {
			limit = maxUploadBytes
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

func withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// cors hanya mengizinkan origin web admin, tanpa wildcard (MASTER.md §5.7).
func cors(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Expose-Headers", "X-Request-Id")
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					h.Set("Access-Control-Allow-Methods", strings.Join([]string{"GET", "POST", "PATCH", "PUT", "DELETE"}, ", "))
					h.Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
					h.Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeaders(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			if secure {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
