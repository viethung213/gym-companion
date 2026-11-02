package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway"
	"github.com/viethung213/gym-companion/internal/gateway/admin"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type testEnvironment struct {
	gatewayURL string
	adminURL   string
	backendURL string
	adminKey   string
	cleanup    func()
}

func setupTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	// 1. Check if external environment (Docker or local compose) is already running
	const defaultGateway = "http://localhost:8000"
	const defaultAdmin = "http://localhost:8001"
	const defaultBackend = "http://localhost:8080"
	const defaultAdminKey = "gym-companion-admin-secret-2026"

	// 1. Opt-in via GATEWAY_E2E_LIVE=true to test against live containers/ports
	if liveEnv := os.Getenv("GATEWAY_E2E_LIVE"); strings.EqualFold(liveEnv, "true") || strings.EqualFold(liveEnv, "1") {
		client := &http.Client{Timeout: 500 * time.Millisecond}
		respGW, errGW := client.Get(defaultGateway + "/health")
		var gwOK bool
		if errGW == nil {
			respGW.Body.Close()
			gwOK = respGW.StatusCode == http.StatusOK
		}

		connBE, errBE := net.DialTimeout("tcp", "127.0.0.1:8080", 500*time.Millisecond)
		var beOK bool
		if errBE == nil {
			_ = connBE.Close()
			beOK = true
		}

		if gwOK && beOK {
			t.Log("Using live running Gateway & Backend environment on :8000, :8001, :8080")
			return &testEnvironment{
				gatewayURL: defaultGateway,
				adminURL:   defaultAdmin,
				backendURL: defaultBackend,
				adminKey:   defaultAdminKey,
				cleanup:    func() {},
			}
		}
	}

	// 2. Otherwise, start an in-process full Gateway + Protected Upstream test harness
	t.Log("Starting in-process Gateway E2E test harness...")
	const internalSecret = "test-internal-secret-for-e2e-2026"

	// Mock Upstream Backend enforcing Zero-Trust X-Internal-Token
	backendMux := http.NewServeMux()
	backendMux.HandleFunc("/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("X-Internal-Token")
		if tokenStr == "" {
			http.Error(w, `{"code":"unauthenticated","message":"missing internal token"}`, http.StatusUnauthorized)
			return
		}
		claims, err := internaltoken.Validate(tokenStr, internalSecret)
		if err != nil {
			http.Error(w, `{"code":"unauthenticated","message":"invalid internal token"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Received-User", claims.UserID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"exercises":[{"id":"ex-1","name":"Bench Press"}]}`))
	})

	backendServer := httptest.NewServer(h2c.NewHandler(backendMux, &http2.Server{}))

	// Create temporary route config pointing to our backend server
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "routes.json")
	routeConfig := fmt.Sprintf(`{
		"allowed_hosts": ["localhost", "127.0.0.1", "testserver"],
		"services": [
			{
				"id": "exercise-service",
				"prefixes": ["/contracts.supporting.exercise."],
				"upstreams": [%q]
			}
		]
	}`, backendServer.URL)

	if err := os.WriteFile(configPath, []byte(routeConfig), 0o600); err != nil {
		t.Fatalf("failed to write test routes config: %v", err)
	}

	gw, err := gateway.NewGateway(gateway.Options{
		ConfigPath:      configPath,
		InternalSecret:  internalSecret,
		TokenTTL:        time.Minute,
		WatchInterval:   100 * time.Millisecond,
		EnforceTLS:      false,
		TraceBufferSize: 500,
	})
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	// Start Gateway Data Plane HTTP Server
	gwListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on gateway port: %v", err)
	}
	gwServer := &http.Server{
		Handler:           h2c.NewHandler(gw.Handler(), &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = gwServer.Serve(gwListener)
	}()

	// Start Admin Server
	adminAdapter := admin.NewAdapter(gw.Telemetry(), gw.ConfigManager(), gw)
	adminSrv := admin.NewServer("0", defaultAdminKey, adminAdapter)
	adminListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on admin port: %v", err)
	}
	go func() {
		_ = adminSrv.Serve(adminListener)
	}()

	env := &testEnvironment{
		gatewayURL: "http://" + gwListener.Addr().String(),
		adminURL:   "http://" + adminListener.Addr().String(),
		backendURL: backendServer.URL,
		adminKey:   defaultAdminKey,
		cleanup: func() {
			_ = adminSrv.Shutdown(context.Background())
			_ = gwServer.Shutdown(context.Background())
			gw.Close()
			backendServer.Close()
		},
	}

	return env
}

func TestGatewayE2E_HealthDataPlane(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(env.gatewayURL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !strings.Contains(string(body), "OK") {
		t.Errorf("got body %q, want it to contain 'OK'", string(body))
	}
}

func TestGatewayE2E_HealthAdminPlane(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(env.adminURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(body)), "ok") {
		t.Errorf("got body %q, want it to contain 'ok'", string(body))
	}
}

func TestGatewayE2E_ZeroTrustBypassBlocked(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	// Calling backend directly without X-Internal-Token MUST be rejected (Zero Trust)
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.backendURL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("direct backend request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("direct access got status %d, want %d (Zero-Trust violation)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGatewayE2E_RoutingAndTokenTranslation(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.gatewayURL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("routed request got status %d, want %d (body: %s)", resp.StatusCode, http.StatusOK, string(body))
	}

	// Verify distributed tracing header was injected
	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = resp.Header.Get("X-Request-ID")
	}
	if reqID == "" {
		t.Error("expected X-Request-ID header in gateway response")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(body), "Bench Press") {
		t.Errorf("got body %q, want it to contain 'Bench Press'", string(body))
	}
}

func TestGatewayE2E_AdminControlPlaneMetrics(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}

	// Send at least 1 request through gateway to generate telemetry
	gwReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.gatewayURL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises",
		strings.NewReader(`{}`),
	)
	gwReq.Header.Set("Content-Type", "application/json")
	gwResp, err := client.Do(gwReq)
	if err != nil {
		t.Fatalf("failed to generate traffic: %v", err)
	}
	gwResp.Body.Close()

	// 1. Verify Admin authentication denial without key
	unauthReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, env.adminURL+"/admin/overview", nil)
	unauthResp, err := client.Do(unauthReq)
	if err != nil {
		t.Fatalf("failed unauth request: %v", err)
	}
	unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("admin request without key got status %d, want %d", unauthResp.StatusCode, http.StatusUnauthorized)
	}

	// 2. Verify Admin overview stats with key
	authReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, env.adminURL+"/admin/overview", nil)
	authReq.Header.Set("X-Admin-Key", env.adminKey)
	authResp, err := client.Do(authReq)
	if err != nil {
		t.Fatalf("failed auth admin request: %v", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("admin overview got status %d, want %d", authResp.StatusCode, http.StatusOK)
	}

	var stats telemetry.OverviewStats
	if err := json.NewDecoder(authResp.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode overview stats: %v", err)
	}
	if stats.TotalRequests < 1 {
		t.Errorf("got TotalRequests %d, want >= 1", stats.TotalRequests)
	}

	// 3. Verify Prometheus metrics endpoint
	promReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, env.adminURL+"/metrics", nil)
	promReq.Header.Set("X-Admin-Key", env.adminKey)
	promResp, err := client.Do(promReq)
	if err != nil {
		t.Fatalf("failed prometheus request: %v", err)
	}
	defer promResp.Body.Close()

	if promResp.StatusCode != http.StatusOK {
		t.Fatalf("metrics got status %d, want %d", promResp.StatusCode, http.StatusOK)
	}
	promBody, _ := io.ReadAll(promResp.Body)
	if !strings.Contains(string(promBody), "gateway_requests_total") {
		t.Errorf("prometheus body missing 'gateway_requests_total':\n%s", string(promBody))
	}
}

func TestGatewayE2E_AdminTraces(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}

	// Generate traffic
	gwReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.gatewayURL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises",
		strings.NewReader(`{}`),
	)
	gwReq.Header.Set("Content-Type", "application/json")
	gwResp, err := client.Do(gwReq)
	if err != nil {
		t.Fatalf("failed to send traffic: %v", err)
	}
	gwResp.Body.Close()

	// Query traces from Admin Control Plane
	traceReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, env.adminURL+"/admin/traces", nil)
	traceReq.Header.Set("X-Admin-Key", env.adminKey)
	traceResp, err := client.Do(traceReq)
	if err != nil {
		t.Fatalf("failed to query traces: %v", err)
	}
	defer traceResp.Body.Close()

	if traceResp.StatusCode != http.StatusOK {
		t.Fatalf("traces query got status %d, want %d", traceResp.StatusCode, http.StatusOK)
	}

	var traces []telemetry.RequestJourney
	if err := json.NewDecoder(traceResp.Body).Decode(&traces); err != nil {
		t.Fatalf("failed to decode traces: %v", err)
	}
	if len(traces) == 0 {
		t.Error("expected at least 1 recorded trace journey, got 0")
	}
}

func TestGatewayE2E_DynamicServiceDiscovery(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Start a new dynamic upstream server
	var dynamicHits atomic.Int32
	dynamicMux := http.NewServeMux()
	dynamicMux.HandleFunc("/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", func(w http.ResponseWriter, r *http.Request) {
		dynamicHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"exercises":[{"id":"dyn-1","name":"Dynamic Exercise"}]}`))
	})

	dynamicServer := httptest.NewServer(h2c.NewHandler(dynamicMux, &http2.Server{}))
	defer dynamicServer.Close()

	// 2. Register with Gateway Admin Control Plane
	regPayload := fmt.Sprintf(`{"service_id":"exercise-service","upstream_url":%q}`, dynamicServer.URL)
	regReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.adminURL+"/admin/discovery/register",
		strings.NewReader(regPayload),
	)
	regReq.Header.Set("X-Admin-Key", env.adminKey)
	regReq.Header.Set("Content-Type", "application/json")

	regResp, err := client.Do(regReq)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer regResp.Body.Close()

	if regResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(regResp.Body)
		t.Fatalf("register expected 200, got %d: %s", regResp.StatusCode, string(body))
	}

	// 3. Query instances
	instReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, env.adminURL+"/admin/discovery/instances", nil)
	instReq.Header.Set("X-Admin-Key", env.adminKey)
	instResp, err := client.Do(instReq)
	if err != nil {
		t.Fatalf("instances query failed: %v", err)
	}
	defer instResp.Body.Close()

	if instResp.StatusCode != http.StatusOK {
		t.Fatalf("instances query got %d", instResp.StatusCode)
	}

	// 4. Send multiple requests through Gateway to verify load balancer hits the newly registered server
	for i := 0; i < 5; i++ {
		gwReq, _ := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			env.gatewayURL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises",
			strings.NewReader(`{}`),
		)
		gwReq.Header.Set("Content-Type", "application/json")
		resp, reqErr := client.Do(gwReq)
		if reqErr == nil {
			resp.Body.Close()
		}
	}

	if dynamicHits.Load() == 0 {
		t.Errorf("expected dynamic server to receive at least 1 request, got %d", dynamicHits.Load())
	}

	// 5. Deregister
	deregPayload := fmt.Sprintf(`{"service_id":"exercise-service","upstream_url":%q}`, dynamicServer.URL)
	deregReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.adminURL+"/admin/discovery/deregister",
		strings.NewReader(deregPayload),
	)
	deregReq.Header.Set("X-Admin-Key", env.adminKey)
	deregReq.Header.Set("Content-Type", "application/json")

	deregResp, err := client.Do(deregReq)
	if err != nil {
		t.Fatalf("deregister failed: %v", err)
	}
	defer deregResp.Body.Close()

	if deregResp.StatusCode != http.StatusOK {
		t.Errorf("deregister expected 200, got %d", deregResp.StatusCode)
	}
}

func TestGatewayE2E_ZeroConfigDynamicRouting(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Start a new module server with a dynamically registered prefix
	var authHits atomic.Int32
	authMux := http.NewServeMux()
	authMux.HandleFunc("/contracts.generic.auth.v1.service.AuthService/Login", func(w http.ResponseWriter, r *http.Request) {
		authHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accessToken":"test-token"}`))
	})

	authServer := httptest.NewServer(h2c.NewHandler(authMux, &http2.Server{}))
	defer authServer.Close()

	// 2. Register dynamically with Prefixes (Method 2: Zero-Config)
	regPayload := fmt.Sprintf(`{
		"service_id": "auth-dynamic-service",
		"upstream_url": %q,
		"prefixes": ["/contracts.generic.auth.v1.service.AuthService/"]
	}`, authServer.URL)

	regReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.adminURL+"/admin/discovery/register",
		strings.NewReader(regPayload),
	)
	regReq.Header.Set("X-Admin-Key", env.adminKey)
	regReq.Header.Set("Content-Type", "application/json")

	regResp, err := client.Do(regReq)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer regResp.Body.Close()

	if regResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(regResp.Body)
		t.Fatalf("register expected 200, got %d: %s", regResp.StatusCode, string(body))
	}

	// 3. Request through Gateway Data Plane
	gwReq, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		env.gatewayURL+"/contracts.generic.auth.v1.service.AuthService/Login",
		strings.NewReader(`{}`),
	)
	gwReq.Header.Set("Content-Type", "application/json")
	gwResp, err := client.Do(gwReq)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer gwResp.Body.Close()

	if gwResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(gwResp.Body)
		t.Fatalf("gateway expected 200, got %d: %s", gwResp.StatusCode, string(body))
	}

	if authHits.Load() != 1 {
		t.Errorf("expected 1 hit on authServer, got %d", authHits.Load())
	}
}
