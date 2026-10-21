package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type UnfollowUserCommand struct {
	FollowerID  string
	FollowingID string
}

func (c UnfollowUserCommand) Validate() error {
	if strings.TrimSpace(c.FollowerID) == "" || strings.TrimSpace(c.FollowingID) == "" {
		return derror.ErrUnauthorized
	}
	if c.FollowerID == c.FollowingID {
		return derror.ErrSelfFollow
	}
	return nil
}

type UnfollowUserHandler struct {
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
	txManager        port.TransactionManager
}

func NewUnfollowUserHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	txManager port.TransactionManager,
) *UnfollowUserHandler {
	return &UnfollowUserHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
		txManager:        txManager,
	}
}

func (h *UnfollowUserHandler) Handle(ctx context.Context, cmd UnfollowUserCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		isFollowing, err := h.followRepo.IsFollowing(txCtx, cmd.FollowerID, cmd.FollowingID)
		if err != nil {
			return fmt.Errorf("check is following: %w", err)
		}
		if !isFollowing {
			return nil
		}

		if err := h.followRepo.Unfollow(txCtx, cmd.FollowerID, cmd.FollowingID); err != nil {
			return fmt.Errorf("unfollow: %w", err)
		}

		if h.userSnapshotRepo != nil {
			if err := h.userSnapshotRepo.UpdateFollowCounts(txCtx, cmd.FollowerID, cmd.FollowingID, -1); err != nil {
				return fmt.Errorf("update follow counts: %w", err)
			}
		}

		return nil
	})
}
