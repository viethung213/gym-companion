package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type UserProfileRepository interface {
	Save(ctx context.Context, profile *aggregate.UserProfile) error
	FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error)
	Update(ctx context.Context, profile *aggregate.UserProfile) error

	FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error)
	FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error)
}
