package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gateway/admin"
	"github.com/viethung213/gym-companion/internal/gateway/middleware"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type mockKeyProvider struct {
	pemStr string
}

func (m *mockKeyProvider) GetPublicKeyPEM(_ context.Context, _ string) (string, error) {
	return m.pemStr, nil
}

func generateRSATestKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	return priv, string(pubPEM)
}

func TestGateway_E2ERoutingAndTokenTranslation(t *testing.T) {
	t.Parallel()

	privKey, pubPEM := generateRSATestKey(t)
	kp := &mockKeyProvider{pemStr: pubPEM}
	internalSecret := "unit-test-gateway-secret-key-12345"

	// Mock Backend Server simulating Core & AI service (wrapped with h2c)
	var receivedInternalToken, receivedReqID string
	backend := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedInternalToken = r.Header.Get(middleware.HeaderInternalToken)
		receivedReqID = r.Header.Get(middleware.HeaderRequestID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}), &http2.Server{}))
	defer backend.Close()

	// Temp config file for Gateway
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"services": [
			{
				"id": "mock-backend",
				"prefixes": ["/"],
				"upstreams": ["` + backend.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    kp,
		InternalSecret: internalSecret,
		TokenTTL:       time.Minute,
		WatchInterval:  50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer gw.Close()

	gatewayServer := httptest.NewServer(gw.Handler())
	defer gatewayServer.Close()

	client := gatewayServer.Client()

	t.Run("public route passes through and receives internal token", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, receivedInternalToken)
		assert.NotEmpty(t, receivedReqID)

		claims, err := internaltoken.Validate(receivedInternalToken, internalSecret)
		require.NoError(t, err)
		assert.Equal(t, "guest", claims.UserID)
		assert.Equal(t, "anonymous", claims.Role)
	})

	t.Run("protected route without token is blocked with 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, gatewayServer.URL+"/contracts.auth.v1.AuthService/GetProfile", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("protected route with valid public JWT is translated to internal token", func(t *testing.T) {
		// Generate valid public JWT
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":  "usr-gold-999",
			"role": "Trainer",
			"exp":  time.Now().Add(time.Hour).Unix(),
		})
		token.Header["kid"] = "test-kid"
		tokenStr, err := token.SignedString(privKey)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, gatewayServer.URL+"/contracts.auth.v1.AuthService/GetProfile", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenStr)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, receivedInternalToken)

		claims, err := internaltoken.Validate(receivedInternalToken, internalSecret)
		require.NoError(t, err)
		assert.Equal(t, "usr-gold-999", claims.UserID)
		assert.Equal(t, "Trainer", claims.Role)
	})
}

func TestGateway_StrictH2C_EnforcesHTTP2Only(t *testing.T) {
	t.Parallel()

	// Pure HTTP/1.1 backend (not wrapped with h2c)
	plainHTTP1Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"http1 only"}`))
	}))
	defer plainHTTP1Backend.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"services": [
			{
				"id": "legacy-service",
				"prefixes": ["/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"],
				"upstreams": ["` + plainHTTP1Backend.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    &mockKeyProvider{},
		InternalSecret: "test-secret",
		TokenTTL:       time.Minute,
		WatchInterval:  50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer gw.Close()

	gwServer := httptest.NewServer(gw.Handler())
	defer gwServer.Close()

	// Client requests a public endpoint that points to a non-h2c upstream
	resp, err := http.Get(gwServer.URL + "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Strict H2C Gateway MUST reject non-h2c upstream with 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGateway_TelemetryAndAdminIntegration(t *testing.T) {
	t.Parallel()

	// Mock Backend Server (wrapped with h2c)
	backend := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}), &http2.Server{}))
	defer backend.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"allowed_hosts": ["localhost", "127.0.0.1"],
		"services": [
			{
				"id": "test-service",
				"prefixes": ["/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"],
				"upstreams": ["` + backend.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:      cfgFile,
		KeyProvider:     &mockKeyProvider{},
		InternalSecret:  "test-secret",
		TokenTTL:        time.Minute,
		WatchInterval:   50 * time.Millisecond,
		TraceBufferSize: 500,
	})
	require.NoError(t, err)
	defer gw.Close()

	gwServer := httptest.NewServer(gw.Handler())
	defer gwServer.Close()

	client := gwServer.Client()

	// 1. Send successful 200 OK request
	resp1, err := client.Get(gwServer.URL + "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises")
	require.NoError(t, err)
	_ = resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// 2. Send 401 Unauthorized request to a protected endpoint
	resp2, err := client.Get(gwServer.URL + "/contracts.core.workout.v1.service.WorkoutService/CreateWorkoutPlan")
	require.NoError(t, err)
	_ = resp2.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)

	// 3. Connect Admin Control Plane handler via Adapter
	adminKey := "test-admin-secret"
	inspector := admin.NewAdapter(gw.Telemetry(), gw.ConfigManager())
	adminHandler := admin.NewHandler(adminKey, inspector).Routes()

	// 4. Test /admin/overview
	reqOverview := httptest.NewRequest(http.MethodGet, "/admin/overview", nil)
	reqOverview.Header.Set(admin.HeaderAdminKey, adminKey)
	recOverview := httptest.NewRecorder()
	adminHandler.ServeHTTP(recOverview, reqOverview)

	assert.Equal(t, http.StatusOK, recOverview.Code)
	ov := inspector.GetOverview()
	assert.GreaterOrEqual(t, ov.TotalRequests, uint64(2))
	assert.GreaterOrEqual(t, ov.Success2xx, uint64(1))
	assert.GreaterOrEqual(t, ov.AuthRejectedCount, uint64(1))

	// 5. Test /admin/traces
	reqTraces := httptest.NewRequest(http.MethodGet, "/admin/traces?limit=10", nil)
	reqTraces.Header.Set(admin.HeaderAdminKey, adminKey)
	recTraces := httptest.NewRecorder()
	adminHandler.ServeHTTP(recTraces, reqTraces)

	assert.Equal(t, http.StatusOK, recTraces.Code)
	traces := inspector.QueryTraces(telemetry.TraceFilter{})
	assert.GreaterOrEqual(t, len(traces), 2)
	assert.Equal(t, http.StatusUnauthorized, traces[0].StatusCode)

	// 6. Test /metrics
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	reqMetrics.Header.Set(admin.HeaderAdminKey, adminKey)
	recMetrics := httptest.NewRecorder()
	adminHandler.ServeHTTP(recMetrics, reqMetrics)

	assert.Equal(t, http.StatusOK, recMetrics.Code)
	assert.Contains(t, recMetrics.Body.String(), "gateway_requests_total")
	assert.Contains(t, recMetrics.Body.String(), "gateway_upstream_requests_total")

	// 7. Test /admin/upstreams
	reqUpstreams := httptest.NewRequest(http.MethodGet, "/admin/upstreams", nil)
	reqUpstreams.Header.Set(admin.HeaderAdminKey, adminKey)
	recUpstreams := httptest.NewRecorder()
	adminHandler.ServeHTTP(recUpstreams, reqUpstreams)

	assert.Equal(t, http.StatusOK, recUpstreams.Code)
	upstreams := inspector.GetUpstreamStats()
	assert.Len(t, upstreams, 1)
	assert.Equal(t, backend.URL, upstreams[0].TargetHost)
	assert.True(t, upstreams[0].Healthy)
}

func TestGateway_EmptyUpstreamReturns503WithoutPanic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"allowed_hosts": ["localhost", "127.0.0.1"],
		"services": [
			{
				"id": "unreachable-service",
				"prefixes": ["/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"],
				"upstreams": ["http://127.0.0.1:59999"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    &mockKeyProvider{},
		InternalSecret: "test-secret",
		TokenTTL:       time.Minute,
		WatchInterval:  50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer gw.Close()

	// Simulate all upstreams being drained/down (Next() returns nil)
	route := gw.MatchRoute("/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises")
	require.NotNil(t, route)
	route.LB.RemoveServer("http://127.0.0.1:59999")

	gwServer := httptest.NewServer(gw.Handler())
	defer gwServer.Close()

	client := gwServer.Client()
	resp, err := client.Get(gwServer.URL + "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Must return 503 Service Unavailable, not panic or crash
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestGateway_CircuitBreaker_TripsAndFastFails(t *testing.T) {
	t.Parallel()

	// 1. Backend server that constantly returns 503
	var backendHits atomic.Int64
	backend := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporarily broken"}`))
	}), &http2.Server{}))
	defer backend.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"allowed_hosts": ["localhost", "127.0.0.1"],
		"services": [
			{
				"id": "failing-service",
				"prefixes": ["/contracts.generic.auth.v1.service.AuthService/"],
				"upstreams": ["` + backend.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    &mockKeyProvider{},
		InternalSecret: "test-secret",
		TokenTTL:       time.Minute,
		WatchInterval:  time.Hour,
	})
	require.NoError(t, err)
	defer gw.Close()

	gwServer := httptest.NewServer(gw.Handler())
	defer gwServer.Close()
	client := gwServer.Client()

	// 2. Send 5 requests -> each causes a 503 from backend -> trips Circuit Breaker (threshold: 5)
	for i := 0; i < 5; i++ {
		resp, reqErr := client.Get(gwServer.URL + "/contracts.generic.auth.v1.service.AuthService/Login")
		require.NoError(t, reqErr)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	assert.Equal(t, int64(5), backendHits.Load(), "Backend should have been hit 5 times before circuit tripped")

	// 3. 6th request: Circuit Breaker is now OPEN! Must Fast-Fail at Gateway without hitting backend!
	resp, reqErr := client.Get(gwServer.URL + "/contracts.generic.auth.v1.service.AuthService/Login")
	require.NoError(t, reqErr)
	_, _ = io.Copy(io.Discard, resp.Body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, int64(5), backendHits.Load(), "Backend should NOT have received the 6th request (Fast-Fail)")
}
