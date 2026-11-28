package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// SyncUserSnapshotCommand carries identity details to sync locally.
type SyncUserSnapshotCommand struct {
	UserID      string
	DisplayName string
	AvatarURL   string
}

// SyncUserSnapshotHandler updates local snapshot to decouple leaderboards from auth/profile schemas.
type SyncUserSnapshotHandler struct {
	snapshotRepo repository.UserSnapshotRepository
}

// NewSyncUserSnapshotHandler constructs SyncUserSnapshotHandler.
func NewSyncUserSnapshotHandler(
	snapshotRepo repository.UserSnapshotRepository,
) *SyncUserSnapshotHandler {
	return &SyncUserSnapshotHandler{snapshotRepo: snapshotRepo}
}

// Handle saves the user snapshot locally.
func (h *SyncUserSnapshotHandler) Handle(ctx context.Context, cmd SyncUserSnapshotCommand) error {
	snapshot := entity.NewUserSnapshot(cmd.UserID, cmd.DisplayName, cmd.AvatarURL, time.Now().UTC())
	if err := h.snapshotRepo.SaveSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("sync user snapshot: %w", err)
	}
	return nil
}
