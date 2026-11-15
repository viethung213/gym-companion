package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
)

type UserSnapshotRepository interface {
	Upsert(ctx context.Context, snapshot *entity.UserSnapshot) error
	UpdateXP(ctx context.Context, userID string, xp int64, syncedAt time.Time) error
	FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error)
	FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error)
}
