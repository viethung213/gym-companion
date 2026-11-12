package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

const (
	defaultUnfollowDebounceHoldDuration = 5 * time.Second
	defaultUnfollowDebounceTimeoutTTL   = 60 * time.Second
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
	debounceBuffer   port.FollowDebounceBuffer
}

// UnfollowUserOption defines functional configuration for UnfollowUserHandler.
type UnfollowUserOption func(*UnfollowUserHandler)

// WithUnfollowDebounceBuffer enables asynchronous debounce buffering via Redis ZSET.
func WithUnfollowDebounceBuffer(buffer port.FollowDebounceBuffer) UnfollowUserOption {
	return func(h *UnfollowUserHandler) {
		h.debounceBuffer = buffer
	}
}

func NewUnfollowUserHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	txManager port.TransactionManager,
	opts ...UnfollowUserOption,
) *UnfollowUserHandler {
	h := &UnfollowUserHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
		txManager:        txManager,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *UnfollowUserHandler) Handle(ctx context.Context, cmd UnfollowUserCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	// 1. Fast-path: Nếu có Debounce Buffer (Redis ZSET 5s hold + 60s TTL)
	if h.debounceBuffer != nil {
		isFollowing, hasPending, pErr := h.debounceBuffer.GetPendingFollow(ctx, cmd.FollowerID, cmd.FollowingID)
		if pErr != nil {
			hasPending = false
		}

		if !hasPending {
			var checkErr error
			isFollowing, checkErr = h.followRepo.IsFollowing(ctx, cmd.FollowerID, cmd.FollowingID)
			if checkErr != nil {
				return fmt.Errorf("check is following: %w", checkErr)
			}
		}

		// Đã không follow rồi thì idempotent return ngay
		if !isFollowing {
			return nil
		}

		if bufErr := h.debounceBuffer.BufferFollow(
			ctx,
			cmd.FollowerID,
			cmd.FollowingID,
			false,
			defaultUnfollowDebounceHoldDuration,
			defaultUnfollowDebounceTimeoutTTL,
		); bufErr != nil {
			return fmt.Errorf("buffer unfollow: %w", bufErr)
		}

		return nil
	}

	// 2. Synchronous Fallback (Khi debounce buffer không được kích hoạt)
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
