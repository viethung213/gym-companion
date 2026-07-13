package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
)

// TokenService defines the application port for handling JWT access/refresh token operations.
type TokenService interface {
	GenerateAccessToken(ctx context.Context, user *aggregate.User, kid string) (string, time.Time, error)
	GenerateRefreshToken(ctx context.Context, user *aggregate.User) (string, time.Time, error)
}
