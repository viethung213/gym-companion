package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// LeaderboardRepository defines queries for the global XP leaderboard and user identity snapshots.
type LeaderboardRepository interface {
	GetGlobalTop(ctx context.Context, limit, offset int) ([]*entity.LeaderboardUser, int64, error)
	GetUserRank(ctx context.Context, userID string) (int64, *entity.LeaderboardUser, error)
	SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error
	GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error)
}
