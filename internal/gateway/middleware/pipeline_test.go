package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

type mockTestKeyProvider struct {
	pubKey *rsa.PublicKey
}

func (m *mockTestKeyProvider) GetPublicKey(_ context.Context, _ string) (*rsa.PublicKey, error) {
	return m.pubKey, nil
}

func TestPipeline_E2E(t *testing.T) {
	t.Parallel()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kp := &mockTestKeyProvider{pubKey: &privKey.PublicKey}

	hf := NewHostFilter()
	hf.SetAllowedHosts([]string{"localhost"})

	pipeline := NewPipeline(PipelineOptions{
		HostFilter:     hf,
		KeyProvider:    kp,
		InternalSecret: "secret",
		TokenTTL:       time.Minute,
		EnforceTLS:     false,
	})
	// Pre-fill keyCache on internal validator for test
	pipeline.jwtVal.keyCache["kid-1"] = &privKey.PublicKey

	var receivedToken, receivedReqID string
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get(HeaderInternalToken)
		receivedReqID = r.Header.Get(HeaderRequestID)
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.Wrap(backendHandler)

	// Create valid user JWT
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  "usr-999",
		"role": "Member",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "kid-1"
	tokenStr, err := token.SignedString(privKey)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/contracts.profile.v1.ProfileService/GetProfile", nil)
	req.Host = "localhost:8000"
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, receivedToken)
	assert.NotEmpty(t, receivedReqID)
}

func TestPipeline_TLSEnforcement(t *testing.T) {
	t.Parallel()

	pipeline := NewPipeline(PipelineOptions{
		InternalSecret: "secret",
		EnforceTLS:     true,
	})

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.Wrap(dummyHandler)

	publicPath := "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"

	// Plain HTTP without X-Forwarded-Proto -> 426 Upgrade Required
	req := httptest.NewRequest(http.MethodGet, "http://example.com"+publicPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUpgradeRequired, rec.Code)

	// HTTPS via X-Forwarded-Proto -> 200 OK
	reqTLS := httptest.NewRequest(http.MethodGet, "http://example.com"+publicPath, nil)
	reqTLS.Header.Set("X-Forwarded-Proto", "https")
	recTLS := httptest.NewRecorder()
	handler.ServeHTTP(recTLS, reqTLS)

	assert.Equal(t, http.StatusOK, recTLS.Code)
	assert.Contains(t, recTLS.Header().Get("Strict-Transport-Security"), "max-age=")
}

func TestPipeline_RejectExternalInternalToken(t *testing.T) {
	t.Parallel()

	secret := "test-internal-secret-2026"
	pipeline := NewPipeline(PipelineOptions{
		InternalSecret: secret,
		TokenTTL:       time.Minute,
		EnforceTLS:     false,
	})

	var backendCalled bool
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.Wrap(backendHandler)

	protectedPath := "/contracts.supporting.profile.v1.service.ProfileService/GetProfile"
	publicPath := "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"

	t.Run("external client sending internal token to protected route is rejected and stripped", func(t *testing.T) {
		backendCalled = false
		tokenStr, err := internaltoken.Generate("svc-ai", "system", "trace-ai-1", time.Minute, secret)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, protectedPath, nil)
		req.Header.Set(HeaderInternalToken, tokenStr)
		req.Header.Set(HeaderRequestID, "trace-ai-99")
		req.Header.Set(HeaderRequestID, "trace-ai-1")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.False(t, backendCalled)
	})

	t.Run("external client sending internal token to public route is rejected and stripped", func(t *testing.T) {
		backendCalled = false
		tokenStr, err := internaltoken.Generate("attacker", "Admin", "trace-hack", time.Minute, secret)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, publicPath, nil)
		req.Header.Set(HeaderInternalToken, tokenStr)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.False(t, backendCalled)
	})

	t.Run("invalid internal token is also rejected with 403", func(t *testing.T) {
		backendCalled = false
		req := httptest.NewRequest(http.MethodPost, protectedPath, nil)
		req.Header.Set(HeaderInternalToken, "invalid-token-value")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.False(t, backendCalled)
	})
}

func TestPipeline_WrapInternal(t *testing.T) {
	t.Parallel()

	secret := "test-internal-secret-2026"
	pipeline := NewPipeline(PipelineOptions{
		InternalSecret: secret,
		TokenTTL:       time.Minute,
		EnforceTLS:     false,
	})

	var (
		backendCalled  bool
		receivedUserID string
		receivedRole   string
		receivedReqID  string
	)
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		receivedUserID = r.Header.Get("X-User-Id")
		receivedRole = r.Header.Get("X-User-Role")
		receivedReqID = r.Header.Get(HeaderRequestID)
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.WrapInternal(backendHandler)
	protectedPath := "/contracts.supporting.profile.v1.service.ProfileService/GetProfile"

	t.Run("valid internal token allows access and injects verified claims", func(t *testing.T) {
		backendCalled = false
		tokenStr, err := internaltoken.Generate("svc-ai", "system", "trace-ai-99", time.Minute, secret)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, protectedPath, nil)
		req.Header.Set(HeaderInternalToken, tokenStr)
		req.Header.Set(HeaderRequestID, "trace-ai-99")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, backendCalled)
		assert.Equal(t, "svc-ai", receivedUserID)
		assert.Equal(t, "system", receivedRole)
		assert.Equal(t, "trace-ai-99", receivedReqID)
	})

	t.Run("missing internal token is rejected with 401 unauthenticated", func(t *testing.T) {
		backendCalled = false
		req := httptest.NewRequest(http.MethodPost, protectedPath, nil)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.False(t, backendCalled)
	})

	t.Run("invalid internal token is rejected with 401 unauthenticated", func(t *testing.T) {
		backendCalled = false
		req := httptest.NewRequest(http.MethodPost, protectedPath, nil)
		req.Header.Set(HeaderInternalToken, "forged.fake.token")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.False(t, backendCalled)
	})

	t.Run("health check passes through without internal token", func(t *testing.T) {
		backendCalled = false
		req := httptest.NewRequest(http.MethodGet, "/health", nil)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, backendCalled)
	})
}
