package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type SyncUserSnapshotCommand struct {
	UserID      string
	DisplayName string
	AvatarURL   string
	SyncedAt    time.Time
}

type SyncUserXPCommand struct {
	UserID   string
	XP       int64
	SyncedAt time.Time
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

func (h *SyncUserSnapshotHandler) HandleXP(ctx context.Context, cmd SyncUserXPCommand) error {
	trimmedID := strings.TrimSpace(cmd.UserID)
	if trimmedID == "" {
		return fmt.Errorf("sync user xp: %w", derror.ErrEmptyUserID)
	}
	xp := cmd.XP
	if xp < 0 {
		xp = 0
	}
	syncedAt := cmd.SyncedAt
	if syncedAt.IsZero() {
		syncedAt = time.Now().UTC()
	}

	if err := h.snapshotRepo.UpdateXP(ctx, trimmedID, xp, syncedAt); err != nil {
		return fmt.Errorf("update user snapshot xp: %w", err)
	}

	return nil
}
