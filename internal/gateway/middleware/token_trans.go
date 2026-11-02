package middleware

import (
	"time"

	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

const (
	// HeaderInternalToken carries the cryptographically signed Zero-Trust token for microservices.
	HeaderInternalToken = "X-Internal-Token"
	// HeaderRequestID carries the distributed tracing request identifier.
	HeaderRequestID = "X-Request-ID"
)

// TokenTranslator handles generating short-lived HMAC-SHA256 tokens for downstream microservices.
type TokenTranslator struct {
	secret string
	ttl    time.Duration
}

// NewTokenTranslator creates a token translator instance.
func NewTokenTranslator(secret string, ttl time.Duration) *TokenTranslator {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &TokenTranslator{
		secret: secret,
		ttl:    ttl,
	}
}

// Generate creates a short-lived internal token for the downstream microservices.
func (t *TokenTranslator) Generate(userID, role, requestID string) (string, error) {
	return internaltoken.Generate(userID, role, requestID, t.ttl, t.secret)
}

// Validate verifies an incoming internal token against the translator's secret.
func (t *TokenTranslator) Validate(tokenStr string) (*internaltoken.InternalClaims, error) {
	return internaltoken.Validate(tokenStr, t.secret)
}
