package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// UserStreakRepository defines storage operations for UserStreak aggregate and streak history.
type UserStreakRepository interface {
	FindByID(ctx context.Context, userID string) (*aggregate.UserStreak, error)
	FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserStreak, error)
	Save(ctx context.Context, userStreak *aggregate.UserStreak) error
	SaveHistory(ctx context.Context, history *entity.StreakHistoryEntry) error
	GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.StreakHistoryEntry, int64, error)
}
