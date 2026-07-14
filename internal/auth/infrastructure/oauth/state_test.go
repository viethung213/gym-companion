package oauth

import (
	"errors"
	"testing"
	"time"
)

func TestState_GenerateAndValidate_Success(t *testing.T) {
	t.Parallel()
	secret := "test-secret-key"
	state, err := GenerateState(secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating state: %v", err)
	}

	if state == "" {
		t.Fatal("expected non-empty state")
	}

	err = ValidateState(state, secret)
	if err != nil {
		t.Fatalf("expected state to be valid, got error: %v", err)
	}
}

func TestState_InvalidSignature(t *testing.T) {
	t.Parallel()
	secret := "test-secret-key"
	state, err := GenerateState(secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating state: %v", err)
	}

	// Try validating with a different secret
	err = ValidateState(state, "wrong-secret-key")
	if !errors.Is(err, ErrInvalidStateSignature) {
		t.Errorf("got error %v, want %v", err, ErrInvalidStateSignature)
	}
}

func TestState_InvalidFormat(t *testing.T) {
	t.Parallel()
	err := ValidateState("invalid.format", "secret")
	if !errors.Is(err, ErrInvalidStateFormat) {
		t.Errorf("got error %v, want %v", err, ErrInvalidStateFormat)
	}
}

func TestState_Expired(t *testing.T) {
	t.Parallel()
	secret := "test-secret-key"
	// Generate a state that has already expired (negative TTL)
	state, err := GenerateState(secret, -1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating state: %v", err)
	}

	err = ValidateState(state, secret)
	if !errors.Is(err, ErrStateExpired) {
		t.Errorf("got error %v, want %v", err, ErrStateExpired)
	}
}
