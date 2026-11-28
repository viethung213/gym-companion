package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

// UserSnapshotRepository stores the locally cached identity (display name, avatar) of users.
type UserSnapshotRepository interface {
	SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error
	GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error)
	// GetSnapshots returns the snapshots found for userIDs keyed by user ID; unknown IDs are absent.
	GetSnapshots(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error)
}
