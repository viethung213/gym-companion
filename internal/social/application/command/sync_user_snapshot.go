package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type SyncUserSnapshotCommand struct {
	UserID    string
	FullName  string
	AvatarURL string
	Role      string
}

func (c SyncUserSnapshotCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	return nil
}

type SyncUserSnapshotHandler struct {
	userSnapshotRepo repository.UserSnapshotRepository
	txManager        port.TransactionManager
}

func NewSyncUserSnapshotHandler(
	userSnapshotRepo repository.UserSnapshotRepository,
	txManager port.TransactionManager,
) *SyncUserSnapshotHandler {
	return &SyncUserSnapshotHandler{
		userSnapshotRepo: userSnapshotRepo,
		txManager:        txManager,
	}
}

func (h *SyncUserSnapshotHandler) Handle(ctx context.Context, cmd SyncUserSnapshotCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	snapshot := entity.NewUserSnapshot(cmd.UserID, cmd.FullName, cmd.AvatarURL, cmd.Role, time.Now().UTC())

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		err := h.userSnapshotRepo.Upsert(txCtx, snapshot)
		if err != nil {
			return fmt.Errorf("sync user snapshot: %w", err)
		}
		return nil
	})
}
