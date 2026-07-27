package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/httpx"
)

// ─── Recoverer ────────────────────────────────────────────────────────────────

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Str("request_id", RequestIDFromContext(r.Context())).
					Interface("panic", rec).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				httpx.WriteError(w, apperror.Internal(nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowed["*"] || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateBucket
	limit    int
	window   time.Duration
}

type rateBucket struct {
	count  int
	resetAt time.Time
}

func newRateLimiter(perMinute int) *rateLimiter {
	rl := &rateLimiter{
		counters: make(map[string]*rateBucket),
		limit:    perMinute,
		window:   time.Minute,
	}
	// Background cleanup every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			rl.mu.Lock()
			now := time.Now()
			for k, b := range rl.counters {
				if now.After(b.resetAt) {
					delete(rl.counters, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.counters[key]
	if !ok || now.After(b.resetAt) {
		rl.counters[key] = &rateBucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

// RateLimit returns a middleware that rate-limits requests by IP.
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	rl := newRateLimiter(perMinute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx >= 0 {
				ip = ip[:idx]
			}
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = strings.Split(fwd, ",")[0]
			}
			if !rl.allow(ip) {
				w.Header().Set("Retry-After", "60")
				httpx.WriteError(w, apperror.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, slow down"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
