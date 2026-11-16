package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type UserProfileRepository interface {
	// Lifecycle & Reconstitution
	Save(ctx context.Context, profile *aggregate.UserProfile) error
	FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error)

	// Surgical persistence methods (eliminate write amplification & cascade overhead)
	UpdateProfile(ctx context.Context, profile *aggregate.UserProfile) error
	UpdateIdentity(ctx context.Context, userID, fullName, avatarURL string) error
	AddPeriodicMetric(ctx context.Context, userID string, metric vo.PeriodicMetric, updatedBio vo.BiologicalMetrics) error
	AddInjury(ctx context.Context, userID string, injury *entity.Injury) error
	RecoverInjury(ctx context.Context, userID, injuryID string) error

	// Query helpers
	FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error)
	FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error)
}
