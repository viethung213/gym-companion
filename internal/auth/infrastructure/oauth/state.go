package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidStateFormat represents invalid format of state string.
var ErrInvalidStateFormat = errors.New("invalid state format")

// ErrInvalidStateSignature represents signature verification failure.
var ErrInvalidStateSignature = errors.New("invalid state signature")

// ErrStateExpired represents state timeout failure.
var ErrStateExpired = errors.New("state token expired")

// GenerateState generates a cryptographically secure, signed state string.
func GenerateState(secret string, ttl time.Duration) (string, error) {
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	randomHex := hex.EncodeToString(randBytes)

	expiresAt := time.Now().Add(ttl).UnixNano()
	expiresHex := fmt.Sprintf("%x", expiresAt)

	payload := expiresHex + "." + randomHex
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signatureHex := hex.EncodeToString(mac.Sum(nil))

	return payload + "." + signatureHex, nil
}

// ValidateState checks if the state string is valid and not expired.
func ValidateState(state string, secret string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		return ErrInvalidStateFormat
	}

	expiresHex := parts[0]
	randomHex := parts[1]
	signatureHex := parts[2]

	payload := expiresHex + "." + randomHex

	// Verify HMAC signature in constant time
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSignature := mac.Sum(nil)

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil || !hmac.Equal(sigBytes, expectedSignature) {
		return ErrInvalidStateSignature
	}

	// Verify expiration
	var expiresAt int64
	if _, err := fmt.Sscanf(expiresHex, "%x", &expiresAt); err != nil {
		return ErrInvalidStateFormat
	}

	if time.Now().UnixNano() > expiresAt {
		return ErrStateExpired
	}

	return nil
}
