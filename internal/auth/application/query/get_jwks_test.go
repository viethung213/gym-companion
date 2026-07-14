//go:build unit

package query

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// ---------------------------------------------------------------------------
// mockKeyRepository
// ---------------------------------------------------------------------------

type mockKeyRepository struct {
	keys []*port.JWKRecord
	err  error
}

func (m *mockKeyRepository) Save(ctx context.Context, key *port.JWKRecord) error {
	return nil
}

func (m *mockKeyRepository) GetActiveKey(ctx context.Context) (*port.JWKRecord, error) {
	return nil, apperror.ErrKeyNotFound
}

func (m *mockKeyRepository) GetAllActiveAndInactiveKeys(ctx context.Context) ([]*port.JWKRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.keys, nil
}

func (m *mockKeyRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	return nil
}

func (m *mockKeyRepository) DeleteExpiredKeys(ctx context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// generateRSAPublicKeyPEM creates a real RSA-2048 PEM public key for test use.
func generateRSAPublicKeyPEM(t *testing.T) (pubPEM string, privKey *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))
	return pubPEM, priv
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGetJWKSHandler_ReturnsValidJWKS(t *testing.T) {
	ctx := context.Background()

	pubPEM, priv := generateRSAPublicKeyPEM(t)

	keyRepo := &mockKeyRepository{
		keys: []*port.JWKRecord{
			{
				ID:           "test-kid-1",
				PublicKeyPEM: pubPEM,
				Algorithm:    "RS256",
				Status:       port.KeyStatusActive,
				CreatedAt:    time.Now(),
				ExpiresAt:    time.Now().Add(24 * time.Hour),
			},
		},
	}

	handler := NewGetJWKSHandler(keyRepo)
	jwks, err := handler.Handle(ctx, GetJWKSQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must return exactly one key
	if got, want := len(jwks), 1; got != want {
		t.Fatalf("got %d keys, want %d", got, want)
	}

	key := jwks[0]

	// Static fields
	if key.Kty != "RSA" {
		t.Errorf("Kty: got %q, want %q", key.Kty, "RSA")
	}
	if key.Use != "sig" {
		t.Errorf("Use: got %q, want %q", key.Use, "sig")
	}
	if key.Alg != "RS256" {
		t.Errorf("Alg: got %q, want %q", key.Alg, "RS256")
	}
	if key.Kid != "test-kid-1" {
		t.Errorf("Kid: got %q, want %q", key.Kid, "test-kid-1")
	}

	// N must be Base64 URL-safe encoding of the RSA modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		t.Fatalf("N is not valid Base64 URL-safe: %v", err)
	}
	decoded := new(big.Int).SetBytes(nBytes)
	if decoded.Cmp(priv.PublicKey.N) != 0 {
		t.Error("N modulus mismatch: decoded value does not match original RSA public key modulus")
	}

	// E must be Base64 URL-safe encoding of 65537 (0x010001)
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		t.Fatalf("E is not valid Base64 URL-safe: %v", err)
	}
	if string(eBytes) != string([]byte{1, 0, 1}) {
		t.Errorf("E bytes: got %v, want [1, 0, 1]", eBytes)
	}
}

func TestGetJWKSHandler_MultipleKeys(t *testing.T) {
	ctx := context.Background()

	pubPEM1, _ := generateRSAPublicKeyPEM(t)
	pubPEM2, _ := generateRSAPublicKeyPEM(t)

	keyRepo := &mockKeyRepository{
		keys: []*port.JWKRecord{
			{ID: "kid-active", PublicKeyPEM: pubPEM1, Status: port.KeyStatusActive, Algorithm: "RS256", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)},
			{ID: "kid-inactive", PublicKeyPEM: pubPEM2, Status: port.KeyStatusInactive, Algorithm: "RS256", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour)},
		},
	}

	handler := NewGetJWKSHandler(keyRepo)
	jwks, err := handler.Handle(ctx, GetJWKSQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both active and inactive keys must appear in JWKS for backwards-compatible token verification
	if got, want := len(jwks), 2; got != want {
		t.Errorf("got %d keys, want %d", got, want)
	}

	kids := map[string]bool{}
	for _, k := range jwks {
		kids[k.Kid] = true
	}
	if !kids["kid-active"] || !kids["kid-inactive"] {
		t.Errorf("expected both keys in JWKS, got: %v", kids)
	}
}

func TestGetJWKSHandler_SkipsInvalidPEM(t *testing.T) {
	ctx := context.Background()

	pubPEM, _ := generateRSAPublicKeyPEM(t)

	keyRepo := &mockKeyRepository{
		keys: []*port.JWKRecord{
			{ID: "valid-kid", PublicKeyPEM: pubPEM, Status: port.KeyStatusActive, Algorithm: "RS256", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)},
			{ID: "bad-kid", PublicKeyPEM: "not-a-valid-pem", Status: port.KeyStatusActive, Algorithm: "RS256", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)},
		},
	}

	handler := NewGetJWKSHandler(keyRepo)
	jwks, err := handler.Handle(ctx, GetJWKSQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The invalid key must be silently skipped, valid one must still appear
	if got, want := len(jwks), 1; got != want {
		t.Errorf("got %d keys, want %d (invalid key should be skipped)", got, want)
	}
	if jwks[0].Kid != "valid-kid" {
		t.Errorf("expected valid-kid, got %s", jwks[0].Kid)
	}
}

func TestGetJWKSHandler_EmptyKeyStore(t *testing.T) {
	ctx := context.Background()

	keyRepo := &mockKeyRepository{keys: []*port.JWKRecord{}}
	handler := NewGetJWKSHandler(keyRepo)

	jwks, err := handler.Handle(ctx, GetJWKSQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jwks) != 0 {
		t.Errorf("expected 0 keys, got %d", len(jwks))
	}
}

func TestGetJWKSHandler_RepoError(t *testing.T) {
	ctx := context.Background()

	keyRepo := &mockKeyRepository{err: apperror.ErrKeyNotFound}
	handler := NewGetJWKSHandler(keyRepo)

	_, err := handler.Handle(ctx, GetJWKSQuery{})
	if err == nil {
		t.Error("expected error when repository fails, got nil")
	}
}
