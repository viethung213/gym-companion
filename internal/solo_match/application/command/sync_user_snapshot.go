package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type SyncUserSnapshotCommand struct {
	UserID      string
	DisplayName string
	AvatarURL   string
	SyncedAt    time.Time
}

type SyncUserSnapshotHandler struct {
	snapshotRepo repository.UserSnapshotRepository
}

func NewSyncUserSnapshotHandler(snapshotRepo repository.UserSnapshotRepository) *SyncUserSnapshotHandler {
	return &SyncUserSnapshotHandler{snapshotRepo: snapshotRepo}
}

func (h *SyncUserSnapshotHandler) Handle(ctx context.Context, cmd SyncUserSnapshotCommand) error {
	snapshot, err := entity.NewUserSnapshot(cmd.UserID, cmd.DisplayName, cmd.AvatarURL, cmd.SyncedAt)
	if err != nil {
		return fmt.Errorf("create user snapshot: %w", err)
	}

	if err := h.snapshotRepo.Upsert(ctx, snapshot); err != nil {
		return fmt.Errorf("upsert user snapshot: %w", err)
	}

	return nil
}
