//go:build unit

package jwt

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/crypto"
)

type mockKeyRepository struct {
	keys []*port.JWKRecord
}

func (m *mockKeyRepository) Save(ctx context.Context, key *port.JWKRecord) error {
	m.keys = append(m.keys, key)
	return nil
}

func (m *mockKeyRepository) GetActiveKey(ctx context.Context) (*port.JWKRecord, error) {
	for _, key := range m.keys {
		if key.Status == port.KeyStatusActive {
			return key, nil
		}
	}
	return nil, apperror.ErrKeyNotFound
}

func (m *mockKeyRepository) GetAllActiveAndInactiveKeys(ctx context.Context) ([]*port.JWKRecord, error) {
	var result []*port.JWKRecord
	for _, key := range m.keys {
		if key.Status == port.KeyStatusActive || key.Status == port.KeyStatusInactive {
			result = append(result, key)
		}
	}
	return result, nil
}

func (m *mockKeyRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	for _, key := range m.keys {
		if key.ID == id {
			key.Status = status
			return nil
		}
	}
	return apperror.ErrKeyNotFound
}

func (m *mockKeyRepository) DeleteExpiredKeys(ctx context.Context) error {
	var active []*port.JWKRecord
	for _, key := range m.keys {
		if !(key.Status == port.KeyStatusInactive && key.ExpiresAt.Before(time.Now())) {
			active = append(active, key)
		}
	}
	m.keys = active
	return nil
}

func generateTestKey(t *testing.T) *port.JWKRecord {
	keyGen := crypto.NewRSAKeyGenerator()
	priv, pub, err := keyGen.Generate(context.Background())
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return &port.JWKRecord{
		ID:            uuid.New().String(),
		PrivateKeyPEM: priv,
		PublicKeyPEM:  pub,
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
}

func validateTokenHelper(t *testing.T, tokenStr string, key *port.JWKRecord) (string, []string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(key.PublicKeyPEM))
		if err != nil {
			return nil, err
		}
		return pubKey, nil
	})
	if err != nil {
		return "", nil, err
	}
	if !token.Valid {
		return "", nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil, fmt.Errorf("invalid claims")
	}

	userID := claims["sub"].(string)
	var roles []string
	if rs, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rs {
			roles = append(roles, r.(string))
		}
	}
	return userID, roles, nil
}

func TestJWTSigner_GenerateAndValidate(t *testing.T) {
	ctx := context.Background()

	key := generateTestKey(t)
	keyRepo := &mockKeyRepository{
		keys: []*port.JWKRecord{key},
	}

	signer := NewJWTSigner(keyRepo, "test-issuer", 5*time.Minute, 24*time.Hour)
	email, _ := vo.NewEmail("user@example.com")
	role, _ := vo.NewRole("user")

	uID, _ := vo.NewUserID("9e0dc099-0df4-436f-b258-004ea10a6234")
	user := aggregate.NewUser(
		uID,
		email,
		"",
		"",
		"john-doe",
		role,
		time.Now(),
		time.Now(),
	)

	tokenStr, expiresAt, err := signer.GenerateAccessToken(ctx, user, key.ID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("expected token string to be not empty")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("expected expiresAt to be in the future")
	}

	gotUserID, gotRoles, err := validateTokenHelper(t, tokenStr, key)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}

	if got, want := gotUserID, user.ID(); got != want {
		t.Errorf("got userID %s, want %s", got, want)
	}

	if len(gotRoles) != 1 || gotRoles[0] != user.Role() {
		t.Errorf("got roles %v, want [%s]", gotRoles, user.Role())
	}
}

func TestJWTSigner_ExpiredToken(t *testing.T) {
	ctx := context.Background()

	key := generateTestKey(t)
	keyRepo := &mockKeyRepository{keys: []*port.JWKRecord{key}}

	signer := NewJWTSigner(keyRepo, "test-issuer", -5*time.Minute, 24*time.Hour)

	email, _ := vo.NewEmail("user@example.com")
	role, _ := vo.NewRole("user")
	uID, _ := vo.NewUserID("9e0dc099-0df4-436f-b258-004ea10a6234")
	user := aggregate.NewUser(
		uID,
		email,
		"",
		"",
		"name",
		role,
		time.Now(),
		time.Now(),
	)

	tokenStr, _, err := signer.GenerateAccessToken(ctx, user, key.ID)
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	_, _, err = validateTokenHelper(t, tokenStr, key)
	if err == nil {
		t.Error("expected validation to fail for expired token")
	} else if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected token expired error, got: %v", err)
	}
}

func TestJWTSigner_InvalidSignature(t *testing.T) {
	ctx := context.Background()

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	signer1 := NewJWTSigner(&mockKeyRepository{keys: []*port.JWKRecord{key1}}, "test-issuer", 5*time.Minute, 24*time.Hour)

	email, _ := vo.NewEmail("u1@mail.com")
	role, _ := vo.NewRole("user")
	uID, _ := vo.NewUserID("9e0dc099-0df4-436f-b258-004ea10a6235")
	user := aggregate.NewUser(uID, email, "", "", "n", role, time.Now(), time.Now())

	tokenStr, _, err := signer1.GenerateAccessToken(ctx, user, key1.ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify using key2 (which is different from key1 used for signing)
	_, _, err = validateTokenHelper(t, tokenStr, key2)
	if err == nil {
		t.Error("expected validation to fail due to signature mismatch")
	}
}
