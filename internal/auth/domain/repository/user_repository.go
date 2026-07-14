package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
)

// UserRepository defines the persistence port for the User aggregate.
type UserRepository interface {
	Create(ctx context.Context, user *aggregate.User) error
	Update(ctx context.Context, user *aggregate.User) error
	FindByID(ctx context.Context, id string) (*aggregate.User, error)
	FindByEmail(ctx context.Context, email string) (*aggregate.User, error)
	FindByGoogleID(ctx context.Context, googleID string) (*aggregate.User, error)
	FindByFacebookID(ctx context.Context, facebookID string) (*aggregate.User, error)
}
