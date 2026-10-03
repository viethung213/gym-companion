package entity

import (
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

// PasswordResetToken represents a one-time token for password reset.
type PasswordResetToken struct {
	token     string
	userID    string
	expiresAt time.Time
	isUsed    bool
	createdAt time.Time
}

// NewPasswordResetToken creates a new PasswordResetToken instance.
func NewPasswordResetToken(token string, userID string, ttl time.Duration) *PasswordResetToken {
	now := time.Now()
	return &PasswordResetToken{
		token:     token,
		userID:    userID,
		expiresAt: now.Add(ttl),
		isUsed:    false,
		createdAt: now,
	}
}

// ReconstitutePasswordResetToken restores the entity from persistence.
func ReconstitutePasswordResetToken(
	token string,
	userID string,
	expiresAt time.Time,
	isUsed bool,
	createdAt time.Time,
) *PasswordResetToken {
	return &PasswordResetToken{
		token:     token,
		userID:    userID,
		expiresAt: expiresAt,
		isUsed:    isUsed,
		createdAt: createdAt,
	}
}

// Use consumes the token. Returns error if expired or already used.
func (t *PasswordResetToken) Use(now time.Time) error {
	if t.isUsed {
		return derror.ErrResetTokenAlreadyUsed
	}
	if now.After(t.expiresAt) {
		return derror.ErrResetTokenExpired
	}
	t.isUsed = true
	return nil
}

// Getters
func (t *PasswordResetToken) Token() string        { return t.token }
func (t *PasswordResetToken) UserID() string       { return t.userID }
func (t *PasswordResetToken) ExpiresAt() time.Time { return t.expiresAt }
func (t *PasswordResetToken) IsUsed() bool         { return t.isUsed }
func (t *PasswordResetToken) CreatedAt() time.Time { return t.createdAt }
