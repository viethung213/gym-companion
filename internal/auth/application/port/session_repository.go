package port

import (
	"context"
	"time"
)

// SessionRecord is a plain data transfer object representing a user session.
type SessionRecord struct {
	Token     string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired checks if the session is past its expiration time.
func (s *SessionRecord) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SessionRepository defines the persistence port for session management.
type SessionRepository interface {
	Save(ctx context.Context, token string, userID string, expiresAt time.Time) error
	FindByToken(ctx context.Context, token string) (*SessionRecord, error)
	Delete(ctx context.Context, token string) error
	DeleteAllByUserID(ctx context.Context, userID string) error
}
