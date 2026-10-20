package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
)

type UserSnapshotRepository interface {
	Upsert(ctx context.Context, snapshot *entity.UserSnapshot) error
	FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error)
	FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error)
}
