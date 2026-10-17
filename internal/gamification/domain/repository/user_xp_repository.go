package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// UserXpRepository defines the storage operations for the UserXp aggregate and history.
type UserXpRepository interface {
	FindByID(ctx context.Context, userID string) (*aggregate.UserXp, error)
	FindByIDForUpdate(ctx context.Context, userID string) (*aggregate.UserXp, error)
	Save(ctx context.Context, userXp *aggregate.UserXp) error
	SaveHistory(ctx context.Context, history *entity.XpHistoryEntry) error
	GetHistory(ctx context.Context, userID string, limit, offset int) ([]*entity.XpHistoryEntry, int64, error)
	HasSession(ctx context.Context, userID, sessionID string) (bool, error)
}
