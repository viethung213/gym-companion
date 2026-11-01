package middleware

import (
	"context"
	"errors"
	"strings"
)

// ContextKey represents a context key type to avoid collisions.
type ContextKey string

const (
	// UserIDKey is the context key for the user ID.
	UserIDKey ContextKey = "userId"
	// UserRoleKey is the context key for the user role.
	UserRoleKey ContextKey = "userRole"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Actor represents an authenticated or guest caller identity in the system.
type Actor struct {
	UserID string
	Role   string
}

// IsAdmin returns true if the actor has the Admin role.
func (a Actor) IsAdmin() bool {
	return strings.EqualFold(a.Role, "Admin")
}

// IsGuest returns true if the actor is an unauthenticated guest.
func (a Actor) IsGuest() bool {
	return a.UserID == "guest" || a.Role == "anonymous" || a.UserID == ""
}

// RequireAuthenticated ensures the request originated from an authenticated user (not a guest).
func RequireAuthenticated(ctx context.Context) (Actor, error) {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" || userID == "guest" {
		return Actor{}, ErrUnauthorized
	}
	role, _ := ctx.Value(UserRoleKey).(string)
	return Actor{
		UserID: userID,
		Role:   role,
	}, nil
}

// RequireAdmin ensures the request originated from an authenticated Admin.
func RequireAdmin(ctx context.Context) (Actor, error) {
	actor, err := RequireAuthenticated(ctx)
	if err != nil {
		return Actor{}, err
	}
	if !actor.IsAdmin() {
		return Actor{}, ErrForbidden
	}
	return actor, nil
}
