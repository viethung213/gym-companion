package middleware

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const (
	defaultLimiterTTL    = 3 * time.Minute
	defaultSweepInterval = 1 * time.Minute
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterRegistry struct {
	mu        sync.Mutex
	limiters  map[string]*limiterEntry
	ttl       time.Duration
	lastSweep time.Time
}

func newRateLimiterRegistry() *rateLimiterRegistry {
	return &rateLimiterRegistry{
		limiters:  make(map[string]*limiterEntry),
		ttl:       defaultLimiterTTL,
		lastSweep: time.Now(),
	}
}

// GetLimiter returns or creates a rate.Limiter for a given key,
// with a specific limit (requests per minute).
func (r *rateLimiterRegistry) GetLimiter(key string, reqPerMin int) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Lazy cleanup sweep to prevent memory leak without unmanaged background goroutines
	if now.Sub(r.lastSweep) > defaultSweepInterval {
		r.sweep(now)
		r.lastSweep = now
	}

	entry, exists := r.limiters[key]
	if !exists {
		limit := rate.Every(time.Minute / time.Duration(reqPerMin))
		// We allow a burst of 10% of the limit or at least 1
		burst := reqPerMin / 10
		if burst < 1 {
			burst = 1
		}
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(limit, burst),
			lastSeen: now,
		}
		r.limiters[key] = entry
	} else {
		entry.lastSeen = now
	}
	return entry.limiter
}

func (r *rateLimiterRegistry) sweep(now time.Time) {
	for key, entry := range r.limiters {
		if now.Sub(entry.lastSeen) > r.ttl {
			delete(r.limiters, key)
		}
	}
}

// UnaryRateLimitInterceptor limits gRPC unary requests based on method rules.
func UnaryRateLimitInterceptor() grpc.UnaryServerInterceptor {
	registry := newRateLimiterRegistry()
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		reqPerMin := getLimitForMethod(info.FullMethod)
		clientKey := getClientKey(ctx, info.FullMethod)

		limiter := registry.GetLimiter(clientKey, reqPerMin)
		if !limiter.Allow() {
			return nil, status.Errorf(
				codes.ResourceExhausted,
				"rate limit exceeded: max %d requests per minute for %s",
				reqPerMin,
				info.FullMethod,
			)
		}

		return handler(ctx, req)
	}
}

func getLimitForMethod(fullMethod string) int {
	// Rule in AGENTS.md: "100 req/phút Onboarding API, 10 req/phút CompleteSession API" (CompleteWorkoutSession)
	if strings.Contains(fullMethod, "Onboarding") {
		return 100
	}
	if strings.Contains(fullMethod, "CompleteWorkoutSession") {
		return 10
	}
	// Default to 100 requests per minute for other APIs
	return 100
}

func getClientKey(ctx context.Context, method string) string {
	// 1. Try User ID from context
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		return userID + ":" + method
	}

	// 2. Fallback to Client IP
	ip := extractClientIP(ctx)
	return ip + ":" + method
}

func extractClientIP(ctx context.Context) string {
	// A. Check gRPC metadata for proxy HTTP headers (X-Forwarded-For, X-Real-IP)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if xff := md.Get("x-forwarded-for"); len(xff) > 0 && xff[0] != "" {
			ips := strings.Split(xff[0], ",")
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
		if xri := md.Get("x-real-ip"); len(xri) > 0 && xri[0] != "" {
			clientIP := strings.TrimSpace(xri[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// B. Peer address from gRPC context (stripped of dynamic client port)
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		addrStr := p.Addr.String()
		host, _, err := net.SplitHostPort(addrStr)
		if err == nil && host != "" {
			return host
		}
		return addrStr
	}

	return "unknown"
}
