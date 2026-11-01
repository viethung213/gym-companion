package internaltoken

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrMissingToken indicates that the token string is empty.
	ErrMissingToken = errors.New("missing internal token")
	// ErrInvalidToken indicates token verification or parsing failure.
	ErrInvalidToken = errors.New("invalid internal token")
	// ErrExpiredToken indicates the internal token TTL has elapsed.
	ErrExpiredToken = errors.New("internal token expired")
	// ErrInvalidClaims indicates that token claims could not be cast to InternalClaims.
	ErrInvalidClaims = errors.New("invalid internal claims")
)

// InternalClaims represents identity and trace propagation data carried in internal service calls.
type InternalClaims struct {
	UserID    string `json:"uid"`
	Role      string `json:"role"`
	RequestID string `json:"rid"`
	jwt.RegisteredClaims
}

// Generate generates an HMAC-SHA256 signed JWT with short-lived TTL for internal network traffic.
func Generate(userID, role, requestID string, ttl time.Duration, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("internal secret cannot be empty")
	}

	now := time.Now()
	claims := InternalClaims{
		UserID:    userID,
		Role:      role,
		RequestID: requestID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign internal token: %w", err)
	}

	return tokenString, nil
}

// Validate parses and validates an internal token against the shared secret and checks TTL.
func Validate(tokenStr, secret string) (*InternalClaims, error) {
	if tokenStr == "" {
		return nil, ErrMissingToken
	}
	if secret == "" {
		return nil, errors.New("internal secret cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &InternalClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing algorithm %v", ErrInvalidToken, t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*InternalClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}
