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
	requestTimeout = 15 * time.Second

	// Batas per akun 120/menit (MASTER.md §5.7). Batas per IP dibuat longgar karena satu
	// kantor bisa berbagi satu IP publik; fungsinya menahan banjir dari klien tak dikenal.
	userLimitPerMinute = 120
	ipLimitPerMinute   = 1200
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

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
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
