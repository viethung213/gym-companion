package middleware

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type rateLimiterRegistry struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

// GetLimiter returns or creates a rate.Limiter for a given key,
// with a specific limit (requests per minute).
func (r *rateLimiterRegistry) GetLimiter(key string, reqPerMin int) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	limiter, exists := r.limiters[key]
	if !exists {
		limit := rate.Every(time.Minute / time.Duration(reqPerMin))
		// We allow a burst of 10% of the limit or at least 1
		burst := reqPerMin / 10
		if burst < 1 {
			burst = 1
		}
		limiter = rate.NewLimiter(limit, burst)
		r.limiters[key] = limiter
	}
	return limiter
}

// UnaryRateLimitInterceptor limits gRPC unary requests based on method rules.
func UnaryRateLimitInterceptor() grpc.UnaryServerInterceptor {
	registry := &rateLimiterRegistry{
		limiters: make(map[string]*rate.Limiter),
	}
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
	// Rule: "100 req/phút Onboarding API, 10 req/phút CompleteSession API"
	if strings.Contains(fullMethod, "Onboarding") {
		return 100
	}
	if strings.Contains(fullMethod, "CompleteSession") {
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
	ip := "unknown"
	if p, ok := peer.FromContext(ctx); ok {
		ip = p.Addr.String()
	}
	return ip + ":" + method
}
