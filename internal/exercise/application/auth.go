// Package application contains Exercise use cases and ports.
package application

import (
	"context"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type actorContextKey struct{}

type Actor struct {
	UserID string
	Roles  []string
}

func ContextWithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func RequireAuthenticated(ctx context.Context) (Actor, error) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	if !ok || actor.UserID == "" {
		return Actor{}, domain.ErrUnauthorized
	}

	return actor, nil
}

func RequireAdmin(ctx context.Context) (Actor, error) {
	actor, err := RequireAuthenticated(ctx)
	if err != nil {
		return Actor{}, err
	}
	if !actor.HasRole("Admin") && !actor.HasRole("ADMIN") {
		return Actor{}, domain.ErrForbidden
	}

	return actor, nil
}

func (a Actor) HasRole(role string) bool {
	for _, candidate := range a.Roles {
		if candidate == role {
			return true
		}
	}

	return false
}
