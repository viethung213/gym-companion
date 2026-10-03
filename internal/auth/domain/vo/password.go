package vo

import (
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

// RawPassword represents a validated plain-text password Value Object.
type RawPassword struct {
	value string
}

// NewRawPassword validates and creates a new RawPassword Value Object.
// Password must be at least 8 characters long.
func NewRawPassword(raw string) (RawPassword, error) {
	if len(raw) < 8 {
		return RawPassword{}, derror.ErrPasswordTooShort
	}
	return RawPassword{value: raw}, nil
}

// NewConfirmedRawPassword validates that raw password matches confirm password and meets requirements.
func NewConfirmedRawPassword(raw, confirm string) (RawPassword, error) {
	if raw != confirm {
		return RawPassword{}, derror.ErrPasswordMismatch
	}
	return NewRawPassword(raw)
}

// Value returns the raw password string.
func (p RawPassword) Value() string {
	return p.value
}

// IsZero checks if the RawPassword is uninitialized.
func (p RawPassword) IsZero() bool {
	return p.value == ""
}

// HashedPassword represents a secure password hash (e.g. Argon2id, bcrypt) Value Object.
type HashedPassword struct {
	value string
}

// NewHashedPassword validates and creates a new HashedPassword Value Object.
func NewHashedPassword(hashed string) (HashedPassword, error) {
	trimmed := strings.TrimSpace(hashed)
	if trimmed == "" {
		return HashedPassword{}, derror.ErrEmptyHashedPassword
	}
	return HashedPassword{value: trimmed}, nil
}

// Value returns the encoded hash string.
func (h HashedPassword) Value() string {
	return h.value
}

// IsZero checks if the HashedPassword is uninitialized.
func (h HashedPassword) IsZero() bool {
	return h.value == ""
}
