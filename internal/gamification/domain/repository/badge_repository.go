package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
)

// BadgeRepository defines persistence operations for user achievement badges.
type BadgeRepository interface {
	FindByUserID(ctx context.Context, userID string) ([]*aggregate.UserBadge, error)
	HasBadge(ctx context.Context, userID, badgeCode string) (bool, error)
	Save(ctx context.Context, badge *aggregate.UserBadge) error
	SaveBatch(ctx context.Context, badges []*aggregate.UserBadge) error
}
