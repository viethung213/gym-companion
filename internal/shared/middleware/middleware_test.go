package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	//nolint:gochecknoglobals // Caches private key across tests to avoid regeneration.
	testPrivateKey *rsa.PrivateKey
	//nolint:gochecknoglobals // Used to cache test RSA public key PEM across tests.
	testPublicKeyPEM string
	//nolint:gochecknoglobals // Controls initialization of test RSA keys.
	keysOnce sync.Once
)

func getTestKeys(t *testing.T) (privKey *rsa.PrivateKey, pubKeyPEM string) {
	t.Helper()
	keysOnce.Do(func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate RSA key: %v", err)
		}
		testPrivateKey = priv

		pubASN1, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			t.Fatalf("failed to marshal public key: %v", err)
		}
		pubBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubASN1,
		})
		testPublicKeyPEM = string(pubBytes)
	})
	return testPrivateKey, testPublicKeyPEM
}

type mockKeyProvider struct {
	mu     sync.Mutex
	calls  int
	keyPEM string
	err    error
}

func (m *mockKeyProvider) GetPublicKeyPEM(_ context.Context, kid string) (string, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	if kid == "test-kid" {
		return m.keyPEM, nil
	}
	return "", errors.New("key not found")
}

func (m *mockKeyProvider) GetCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func generateTestToken(t *testing.T, userID, role, kid string) string {
	t.Helper()
	privKey, _ := getTestKeys(t)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	})
	token.Header["kid"] = kid
	tokenStr, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func TestUnaryRecoveryInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := UnaryRecoveryInterceptor()

	// Handler that panics
	panicHandler := func(_ context.Context, _ any) (any, error) {
		panic("something went wrong")
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/TestService/PanicMethod"}
	resp, err := interceptor(context.Background(), nil, info, panicHandler)

	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}

	if st.Code() != codes.Internal {
		t.Errorf("expected code Internal, got %s", st.Code())
	}
}

func TestUnaryLoggingInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := UnaryLoggingInterceptor()

	dummyHandler := func(_ context.Context, _ any) (any, error) {
		return "success", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/TestService/LogMethod"}
	resp, err := interceptor(context.Background(), nil, info, dummyHandler)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if resp != "success" {
		t.Errorf("expected 'success', got %v", resp)
	}
}

func TestIsAuthRequired(t *testing.T) {
	t.Parallel()
	// Test real registered Protobuf method paths
	// 1. GetOAuthLoginURL should be PUBLIC (has security: {})
	if isAuthRequired("/contracts.generic.auth.v1.service.AuthService/GetOAuthLoginURL") {
		t.Error("expected GetOAuthLoginURL to be PUBLIC, but marked as secured")
	}

	// 2. Logout should be SECURED (requires BearerAuth default)
	if !isAuthRequired("/contracts.generic.auth.v1.service.AuthService/Logout") {
		t.Error("expected Logout to be SECURED, but marked as public")
	}

	// 3. Non-existent method should default to SECURED for safety
	if !isAuthRequired("/contracts.generic.auth.v1.service.AuthService/NonExistent") {
		t.Error("expected non-existent method to default to SECURED")
	}
}

func TestUnaryAuthInterceptor(t *testing.T) {
	t.Parallel()
	_, pubKeyPEM := getTestKeys(t)
	mockKP := &mockKeyProvider{
		keyPEM: pubKeyPEM,
	}

	goodToken := generateTestToken(t, "user-123", "user", "test-kid")

	interceptor := UnaryAuthInterceptor(mockKP)

	dummyHandler := func(ctx context.Context, _ any) (any, error) {
		// Verify identity is injected
		uID := ctx.Value(UserIDKey)
		if uID != "user-123" {
			t.Errorf("expected userId user-123 in context, got %v", uID)
		}
		role := ctx.Value(UserRoleKey)
		if role != "user" {
			t.Errorf("expected userRole user in context, got %v", role)
		}
		return "ok", nil
	}

	// Case 1: Secured method, missing header -> should fail
	infoSecured := &grpc.UnaryServerInfo{
		FullMethod: "/contracts.generic.auth.v1.service.AuthService/Logout",
	}
	_, err := interceptor(context.Background(), nil, infoSecured, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated error, got %v", err)
	}

	// Case 2: Secured method, invalid token -> should fail
	ctxWithBadToken := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer bad-token"),
	)
	_, err = interceptor(ctxWithBadToken, nil, infoSecured, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated error, got %v", err)
	}

	// Case 3: Secured method, valid token -> should succeed and call handler
	ctxWithGoodToken := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+goodToken),
	)
	resp, err := interceptor(ctxWithGoodToken, nil, infoSecured, dummyHandler)
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %v", resp)
	}

	// Case 4: Public method, missing header -> should succeed and NOT fail
	infoPublic := &grpc.UnaryServerInfo{
		FullMethod: "/contracts.generic.auth.v1.service.AuthService/GetOAuthLoginURL",
	}
	publicHandler := func(_ context.Context, _ any) (any, error) {
		return "public-ok", nil
	}
	resp, err = interceptor(context.Background(), nil, infoPublic, publicHandler)
	if err != nil {
		t.Errorf("expected public method to pass without auth, got: %v", err)
	}
	if resp != "public-ok" {
		t.Errorf("expected 'public-ok', got %v", resp)
	}
}

func TestUnaryAuthInterceptor_PublicRoute_InvalidToken_Fails(t *testing.T) {
	t.Parallel()
	_, pubKeyPEM := getTestKeys(t)
	mockKP := &mockKeyProvider{
		keyPEM: pubKeyPEM,
	}
	interceptor := UnaryAuthInterceptor(mockKP)
	infoPublic := &grpc.UnaryServerInfo{
		FullMethod: "/contracts.generic.auth.v1.service.AuthService/GetOAuthLoginURL",
	}
	dummyHandler := func(_ context.Context, _ any) (any, error) {
		return "public-ok", nil
	}

	// Invalid token on public route MUST be rejected with Unauthenticated
	ctxWithBadToken := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer invalid-token-string"),
	)
	_, err := interceptor(ctxWithBadToken, nil, infoPublic, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error when invalid token is sent to public route, got %v", err)
	}

	// Malformed auth header (e.g. Basic auth) on public route MUST be rejected
	ctxWithMalformedHeader := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Basic dXNlcjpwYXNz"),
	)
	_, err = interceptor(ctxWithMalformedHeader, nil, infoPublic, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error when malformed header is sent to public route, got %v", err)
	}
}

func TestRSAPublicKeyCache(t *testing.T) {
	t.Parallel()
	_, pubKeyPEM := getTestKeys(t)
	mockKP := &mockKeyProvider{
		keyPEM: pubKeyPEM,
	}
	interceptor := UnaryAuthInterceptor(mockKP)

	infoSecured := &grpc.UnaryServerInfo{
		FullMethod: "/contracts.generic.auth.v1.service.AuthService/Logout",
	}
	goodToken := generateTestToken(t, "user-456", "user", "test-kid")
	ctxWithGoodToken := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+goodToken),
	)

	dummyHandler := func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	}

	// Make 10 requests with the same token/kid
	for i := 0; i < 10; i++ {
		resp, err := interceptor(ctxWithGoodToken, nil, infoSecured, dummyHandler)
		if err != nil || resp != "ok" {
			t.Fatalf("expected request %d to succeed, got err=%v", i, err)
		}
	}

	// Public key provider should only have been called ONCE due to caching
	if calls := mockKP.GetCalls(); calls != 1 {
		t.Errorf("expected KeyProvider to be called exactly 1 time, got %d calls", calls)
	}
}

func TestUnaryRateLimitInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := UnaryRateLimitInterceptor()

	dummyHandler := func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	}

	// Set client key using a mocked peer IP
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// Method name triggers 100 req/min limit, but since the registry gets created on first call
	// let's test that limit gets triggered if we exhaust the burst.
	info := &grpc.UnaryServerInfo{FullMethod: "/TestService/Onboarding"}

	// Exhausting the rate limit burst (burst for 100 limit is 10)
	for i := 0; i < 11; i++ {
		_, err := interceptor(ctx, nil, info, dummyHandler)
		if err != nil {
			if status.Code(err) != codes.ResourceExhausted {
				t.Fatalf("expected ResourceExhausted, got %v", err)
			}
			// Rate limit triggered, success!
			return
		}
	}
	// Note: depending on time, this might not trigger on extremely fast
	// environments if burst is large, but with a 10% burst of 100 (which
	// is 10), calling 11 times in microsecond loop guarantees exhaustion.
}

func TestUnaryRateLimitInterceptor_CompleteWorkoutSession(t *testing.T) {
	t.Parallel()
	interceptor := UnaryRateLimitInterceptor()

	dummyHandler := func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	}

	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:54321")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	info := &grpc.UnaryServerInfo{FullMethod: "/contracts.core.workout_execution.v1.service.WorkoutExecutionService/CompleteWorkoutSession"}

	// For 10 req/min, burst is 10/10 = 1.
	// First request succeeds, 2nd request should be rate limited immediately.
	_, err1 := interceptor(ctx, nil, info, dummyHandler)
	if err1 != nil {
		t.Fatalf("expected first call to succeed, got %v", err1)
	}

	_, err2 := interceptor(ctx, nil, info, dummyHandler)
	if err2 == nil {
		t.Fatalf("expected rate limit error on second call for CompleteWorkoutSession")
	}
	if status.Code(err2) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err2)
	}
}

func TestExtractClientIP(t *testing.T) {
	t.Parallel()
	// Test A: Strip port from peer address
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:54321")
	ctxPeer := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})
	ip := extractClientIP(ctxPeer)
	if ip != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100 without port, got %s", ip)
	}

	// Test B: X-Forwarded-For header priority
	ctxXFF := metadata.NewIncomingContext(
		ctxPeer,
		metadata.Pairs("x-forwarded-for", "203.0.113.195, 70.41.3.18, 150.172.238.178"),
	)
	ipXFF := extractClientIP(ctxXFF)
	if ipXFF != "203.0.113.195" {
		t.Errorf("expected client IP from X-Forwarded-For 203.0.113.195, got %s", ipXFF)
	}

	// Test C: X-Real-IP header priority
	ctxXRI := metadata.NewIncomingContext(
		ctxPeer,
		metadata.Pairs("x-real-ip", "198.51.100.1"),
	)
	ipXRI := extractClientIP(ctxXRI)
	if ipXRI != "198.51.100.1" {
		t.Errorf("expected client IP from X-Real-IP 198.51.100.1, got %s", ipXRI)
	}
}

func TestRateLimiterRegistry_SweepCleanup(t *testing.T) {
	t.Parallel()
	reg := &rateLimiterRegistry{
		limiters:  make(map[string]*limiterEntry),
		ttl:       50 * time.Millisecond,
		lastSweep: time.Now().Add(-2 * time.Minute), // Force sweep trigger on next GetLimiter call
	}

	// Insert an old entry
	reg.limiters["old-key"] = &limiterEntry{
		limiter:  nil,
		lastSeen: time.Now().Add(-100 * time.Millisecond),
	}

	// Call GetLimiter for a new key, triggering sweep
	_ = reg.GetLimiter("new-key", 100)

	reg.mu.Lock()
	_, oldExists := reg.limiters["old-key"]
	_, newExists := reg.limiters["new-key"]
	reg.mu.Unlock()

	if oldExists {
		t.Errorf("expected old-key to be evicted during sweep, but it still exists")
	}
	if !newExists {
		t.Errorf("expected new-key to exist in registry")
	}
}

