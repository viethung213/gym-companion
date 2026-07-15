//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/shared/database"
)

// getTestDB creates a connection to the test database for E2E verification.
func getTestDB(t *testing.T) *gorm.DB {
	sqlDB, err := database.GetRegistry().GetPool("auth")
	if err != nil {
		t.Fatalf("E2E failed to initialize auth database pool from monolith registry: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("E2E failed to wrap database in gorm: %v", err)
	}

	return db
}

// truncateTables clears all tables in the auth schema between tests.
func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE auth.users CASCADE")
	db.Exec("TRUNCATE TABLE auth.jwk_keys CASCADE")
	db.Exec("TRUNCATE TABLE auth.sessions CASCADE")
	db.Exec("TRUNCATE TABLE auth.outbox CASCADE")
}

// startE2ETestServer spins up the full gRPC and HTTP Gateway servers on random ports.
// Returns HTTP base URL, GORM db instance, and a cleanup function.
func startE2ETestServer(t *testing.T) (string, *gorm.DB, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	// 1. Database Connection via Monolith Registry
	rawSQL, err := database.GetRegistry().GetPool("auth")
	if err != nil {
		cancel()
		t.Fatalf("Failed to retrieve connection pool: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: rawSQL,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		cancel()
		t.Fatalf("Failed to wrap DB in GORM: %v", err)
	}
	truncateTables(db)

	// 2. Start gRPC Server on free port
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("Failed to listen on free port for gRPC: %v", err)
	}
	grpcAddr := grpcListener.Addr().String()
	grpcServer := grpc.NewServer()

	// 3. Start HTTP Gateway on free port
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("Failed to listen on free port for HTTP: %v", err)
	}
	httpAddr := httpListener.Addr().String()

	// 4. Bootstrap Auth module via Initialize
	deps := auth.ModuleDeps{
		DB:         rawSQL,
		GRPCServer: grpcServer,
	}
	shutdown, err := auth.Initialize(ctx, deps)
	if err != nil {
		cancel()
		t.Fatalf("Failed to initialize Auth module: %v", err)
	}

	// Run gRPC server in background
	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil && err != grpc.ErrServerStopped {
			log.Printf("gRPC server run error: %v", err)
		}
	}()

	// Register Gateway
	gwmux := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err = auth.RegisterGateway(ctx, gwmux, grpcAddr, dialOpts)
	if err != nil {
		cancel()
		t.Fatalf("Failed to register Auth gRPC-Gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", gwmux)
	httpServer := &http.Server{Handler: mux}
	// Run HTTP server in background
	go func() {
		if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP Gateway run error: %v", err)
		}
	}()

	// Cleanup function
	cleanup := func() {
		cancel()
		shutdown()
		grpcServer.GracefulStop()
		_ = httpServer.Shutdown(context.Background())
		_ = grpcListener.Close()
		_ = httpListener.Close()
		truncateTables(db)
	}

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	return baseURL, db, cleanup
}

// oauthMockTransport intercepts outgoing HTTP requests to Google and Facebook OAuth APIs
type oauthMockTransport struct {
	underlying http.RoundTripper
}

func (m *oauthMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. Google OAuth endpoints mock
	if req.URL.Host == "oauth2.googleapis.com" && req.URL.Path == "/token" {
		respBody := `{"access_token": "google-mock-access-token"}`
		return m.mockResponse(http.StatusOK, respBody)
	}

	if req.URL.Host == "www.googleapis.com" && req.URL.Path == "/oauth2/v2/userinfo" {
		respBody := `{
			"id": "11223344556677889900",
			"email": "google-e2e-user@example.com",
			"name": "Google E2E User"
		}`
		return m.mockResponse(http.StatusOK, respBody)
	}

	// 2. Facebook OAuth endpoints mock
	if req.URL.Host == "graph.facebook.com" && strings.HasPrefix(req.URL.Path, "/v12.0/oauth/access_token") {
		respBody := `{"access_token": "facebook-mock-access-token"}`
		return m.mockResponse(http.StatusOK, respBody)
	}

	if req.URL.Host == "graph.facebook.com" && req.URL.Path == "/me" {
		respBody := `{
			"id": "22334455667788990011",
			"email": "facebook-e2e-user@example.com",
			"name": "Facebook E2E User"
		}`
		return m.mockResponse(http.StatusOK, respBody)
	}

	return m.underlying.RoundTrip(req)
}

func (m *oauthMockTransport) mockResponse(statusCode int, body string) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(statusCode)
	_, _ = rec.WriteString(body)
	res := rec.Result()
	return res, nil
}

// setupOAuthMock intercepts outgoing OAuth client calls using custom RoundTripper
func setupOAuthMock() func() {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &oauthMockTransport{underlying: originalTransport}

	return func() {
		http.DefaultTransport = originalTransport
	}
}
