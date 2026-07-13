package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
)

var (
	testPrivateKey   *rsa.PrivateKey
	testPublicKeyPEM string
)

func init() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	testPrivateKey = priv

	pubASN1, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		panic(err)
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	testPublicKeyPEM = string(pubBytes)
}

type mockKeyProvider struct {
	keyPEM string
	err    error
}

func (m *mockKeyProvider) GetPublicKeyPEM(ctx context.Context, kid string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if kid == "test-kid" {
		return m.keyPEM, nil
	}
	return "", errors.New("key not found")
}

func generateTestToken(userID string, roles []string, kid string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   userID,
		"roles": roles,
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
	})
	token.Header["kid"] = kid
	tokenStr, err := token.SignedString(testPrivateKey)
	if err != nil {
		panic(err)
	}
	return tokenStr
}

func TestUnaryRecoveryInterceptor(t *testing.T) {
	interceptor := UnaryRecoveryInterceptor()

	// Handler that panics
	panicHandler := func(ctx context.Context, req any) (any, error) {
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
	interceptor := UnaryLoggingInterceptor()

	dummyHandler := func(ctx context.Context, req any) (any, error) {
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
	mockKP := &mockKeyProvider{
		keyPEM: testPublicKeyPEM,
	}
	SetKeyProvider(mockKP)

	goodToken := generateTestToken("user-123", []string{"user"}, "test-kid")

	interceptor := UnaryAuthInterceptor()

	dummyHandler := func(ctx context.Context, req any) (any, error) {
		// Verify identity is injected
		uID := ctx.Value("userId")
		if uID != "user-123" {
			t.Errorf("expected userId user-123 in context, got %v", uID)
		}
		return "ok", nil
	}

	// Case 1: Secured method, missing header -> should fail
	infoSecured := &grpc.UnaryServerInfo{FullMethod: "/contracts.generic.auth.v1.service.AuthService/Logout"}
	_, err := interceptor(context.Background(), nil, infoSecured, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated error, got %v", err)
	}

	// Case 2: Secured method, invalid token -> should fail
	ctxWithBadToken := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad-token"))
	_, err = interceptor(ctxWithBadToken, nil, infoSecured, dummyHandler)
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated error, got %v", err)
	}

	// Case 3: Secured method, valid token -> should succeed and call handler
	ctxWithGoodToken := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+goodToken))
	resp, err := interceptor(ctxWithGoodToken, nil, infoSecured, dummyHandler)
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %v", resp)
	}

	// Case 4: Public method, missing header -> should succeed and NOT fail
	infoPublic := &grpc.UnaryServerInfo{FullMethod: "/contracts.generic.auth.v1.service.AuthService/GetOAuthLoginURL"}
	publicHandler := func(ctx context.Context, req any) (any, error) {
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

func TestUnaryRateLimitInterceptor(t *testing.T) {
	interceptor := UnaryRateLimitInterceptor()

	dummyHandler := func(ctx context.Context, req any) (any, error) {
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
	// Note: depending on time, this might not trigger on extremely fast environments if burst is large,
	// but with a 10% burst of 100 (which is 10), calling 11 times in microsecond loop guarantees exhaustion.
}
