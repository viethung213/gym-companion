package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

type UserSnapshotRepository interface {
	GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error)
	GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error)
	Upsert(ctx context.Context, user *entity.UserSnapshot) error
	UpdateRole(ctx context.Context, userID, newRole string) error
	SearchUsers(ctx context.Context, query string, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error)
}
