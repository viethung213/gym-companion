package jwt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
)

// JWTSigner implements port.TokenService using RS256 algorithm.
type JWTSigner struct {
	keyRepo    port.KeyRepository
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// Compile-time interface verification
var _ port.TokenService = (*JWTSigner)(nil)

// NewJWTSigner creates a new instance of JWTSigner.
func NewJWTSigner(keyRepo port.KeyRepository, issuer string, accessTTL time.Duration, refreshTTL time.Duration) *JWTSigner {
	return &JWTSigner{
		keyRepo:    keyRepo,
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GenerateAccessToken signs a new JWT token using the private key matching the given kid.
func (s *JWTSigner) GenerateAccessToken(ctx context.Context, user *aggregate.User, kid string) (string, time.Time, error) {
	// 1. Fetch key pair
	keys, err := s.keyRepo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("get keys for signing: %w", err)
	}

	var targetKey *port.JWKRecord
	for _, key := range keys {
		if key.ID == kid {
			targetKey = key
			break
		}
	}

	if targetKey == nil {
		return "", time.Time{}, fmt.Errorf("signing key %s not found: %w", kid, apperror.ErrKeyNotFound)
	}

	// 2. Parse private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(targetKey.PrivateKeyPEM))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse rsa private key: %w", err)
	}

	// 3. Create claims
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)
	claims := jwt.MapClaims{
		"iss":   s.issuer,
		"sub":   user.ID(),
		"roles": []string{user.Role()},
		"iat":   now.Unix(),
		"exp":   expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	// 4. Sign token
	tokenStr, err := token.SignedString(privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}

	return tokenStr, expiresAt, nil
}

// GenerateRefreshToken generates a secure random refresh token string.
func (s *JWTSigner) GenerateRefreshToken(ctx context.Context, user *aggregate.User) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, fmt.Errorf("read random bytes: %w", err)
	}
	tokenStr := hex.EncodeToString(b)
	expiresAt := time.Now().Add(s.refreshTTL)
	return tokenStr, expiresAt, nil
}
