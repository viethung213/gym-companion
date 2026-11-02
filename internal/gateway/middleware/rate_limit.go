package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
	"golang.org/x/time/rate"
)

const (
	defaultLimiterTTL    = 3 * time.Minute
	defaultSweepInterval = 1 * time.Minute
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages in-memory token bucket rate limiting for the API Gateway.
type RateLimiter struct {
	mu        sync.Mutex
	limiters  map[string]*limiterEntry
	ttl       time.Duration
	lastSweep time.Time
}

// NewRateLimiter initializes a rate limiter with memory sweep cleanup.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters:  make(map[string]*limiterEntry),
		ttl:       defaultLimiterTTL,
		lastSweep: time.Now(),
	}
}

// GetLimiter returns or creates a rate.Limiter for a given key with requests per minute.
func (rl *RateLimiter) GetLimiter(key string, reqPerMin int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastSweep) > defaultSweepInterval {
		rl.sweep(now)
		rl.lastSweep = now
	}

	entry, exists := rl.limiters[key]
	if !exists {
		limit := rate.Every(time.Minute / time.Duration(reqPerMin))
		burst := reqPerMin / 10
		if burst < 1 {
			burst = 1
		}
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(limit, burst),
			lastSeen: now,
		}
		rl.limiters[key] = entry
	} else {
		entry.lastSeen = now
	}

	return entry.limiter
}

func (rl *RateLimiter) sweep(now time.Time) {
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastSeen) > rl.ttl {
			delete(rl.limiters, key)
		}
	}
}

// GetLimitForPath determines rate limits based on system conventions:
// 100 req/min for Onboarding, 10 req/min for CompleteWorkoutSession, 100 req/min default.
func GetLimitForPath(path string) int {
	if strings.Contains(path, "Onboarding") {
		return 100
	}
	if strings.Contains(path, "CompleteWorkoutSession") {
		return 10
	}
	return 100
}

// ExtractClientIP retrieves client IP securely without trusting spoofable proxy headers.
// If Cloudflare/Cloud edge is in front, it reads CF-Connecting-IP.
// Otherwise, it extracts directly from the TCP socket (RemoteAddr) to prevent IP spoofing attacks.
func ExtractClientIP(r *http.Request) string {
	// 1. Nếu có Cloud (như Cloudflare) đứng trước:
	// Đọc header CF-Connecting-IP (do Cloud đóng dấu IP thật của người dùng, hacker không thể fake được)
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		return cfIP
	}

	// 2. Nếu chạy thẳng VPS (không qua Cloud):
	// Lấy trực tiếp IP từ kết nối mạng của hệ điều hành (RemoteAddr)
	// Bỏ hẳn X-Forwarded-For / X-Real-IP để chống tấn công giả mạo IP
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// Wrap returns an http.Handler that enforces rate limiting per client IP & endpoint.
func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		clientIP := ExtractClientIP(r)
		key := clientIP + ":" + path

		limit := GetLimitForPath(path)
		limiter := rl.GetLimiter(key, limit)

		if !limiter.Allow() {
			gwerror.RateLimited(w, r, "Rate limit exceeded. Please try again later.")
			return
		}

		next.ServeHTTP(w, r)
	})
}
