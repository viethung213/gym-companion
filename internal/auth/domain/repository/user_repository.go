package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
)

// ListUsersFilter specifies criteria for querying users.
type ListUsersFilter struct {
	Role   string
	Status string
	Search string
	Offset int
	Limit  int
}

// UserRepository defines the persistence port for the User aggregate.
type UserRepository interface {
	Create(ctx context.Context, user *aggregate.User) error
	Update(ctx context.Context, user *aggregate.User) error
	FindByID(ctx context.Context, id string) (*aggregate.User, error)
	FindByIdentity(ctx context.Context, identityType string, identifier string) (*aggregate.User, error)
	List(ctx context.Context, filter ListUsersFilter) ([]*aggregate.User, int, error)
}
