package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter()
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Wrap(dummyHandler)

	// Burst test for CompleteWorkoutSession (limit = 10 req/min, burst = 1)
	req := httptest.NewRequest(http.MethodPost, "/contracts.workout.v1.WorkoutService/CompleteWorkoutSession", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// First request succeeds
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Immediate second request exceeds burst limit -> 429 Too Many Requests
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
}

func TestRateLimiter_ExtractClientIP(t *testing.T) {
	t.Parallel()

	// 1. From Cloudflare CF-Connecting-IP
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("CF-Connecting-IP", "203.0.113.195")
	req1.RemoteAddr = "10.0.0.1:12345"
	assert.Equal(t, "203.0.113.195", ExtractClientIP(req1))

	// 2. Hacker attempts to spoof IP via X-Forwarded-For or X-Real-IP -> MUST be ignored, falls back to RemoteAddr
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Forwarded-For", "8.8.8.8, 1.1.1.1")
	req2.Header.Set("X-Real-IP", "9.9.9.9")
	req2.RemoteAddr = "198.51.100.1:54321"
	assert.Equal(t, "198.51.100.1", ExtractClientIP(req2), "X-Forwarded-For and X-Real-IP must be ignored to prevent spoofing")

	// 3. From RemoteAddr host:port
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "192.0.2.1:54321"
	assert.Equal(t, "192.0.2.1", ExtractClientIP(req3))
}
