package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

const (
	defaultFollowDebounceHoldDuration = 5 * time.Second
	defaultFollowDebounceTimeoutTTL   = 60 * time.Second
)

type FollowUserCommand struct {
	FollowerID  string
	FollowingID string
}

func (c FollowUserCommand) Validate() error {
	if strings.TrimSpace(c.FollowerID) == "" || strings.TrimSpace(c.FollowingID) == "" {
		return derror.ErrUnauthorized
	}
	if c.FollowerID == c.FollowingID {
		return derror.ErrSelfFollow
	}
	return nil
}

type FollowUserHandler struct {
	followRepo       repository.FollowRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventRecorder    port.EventRecorder
	txManager        port.TransactionManager
	debounceBuffer   port.FollowDebounceBuffer
}

// FollowUserOption defines functional configuration for FollowUserHandler.
type FollowUserOption func(*FollowUserHandler)

// WithFollowDebounceBuffer enables asynchronous debounce buffering via Redis ZSET.
func WithFollowDebounceBuffer(buffer port.FollowDebounceBuffer) FollowUserOption {
	return func(h *FollowUserHandler) {
		h.debounceBuffer = buffer
	}
}

func NewFollowUserHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventRecorder port.EventRecorder,
	txManager port.TransactionManager,
	opts ...FollowUserOption,
) *FollowUserHandler {
	h := &FollowUserHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventRecorder:    eventRecorder,
		txManager:        txManager,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *FollowUserHandler) Handle(ctx context.Context, cmd FollowUserCommand) error {
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

		// Đã follow rồi thì idempotent return ngay
		if isFollowing {
			return nil
		}

		if bufErr := h.debounceBuffer.BufferFollow(
			ctx,
			cmd.FollowerID,
			cmd.FollowingID,
			true,
			defaultFollowDebounceHoldDuration,
			defaultFollowDebounceTimeoutTTL,
		); bufErr != nil {
			return fmt.Errorf("buffer follow: %w", bufErr)
		}

		return nil
	}

	// 2. Synchronous Fallback (Khi debounce buffer không được kích hoạt)
	follow, err := aggregate.NewFollow(
		uuid.New().String(),
		cmd.FollowerID,
		cmd.FollowingID,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("create follow aggregate: %w", err)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		isFollowing, checkErr := h.followRepo.IsFollowing(txCtx, cmd.FollowerID, cmd.FollowingID)
		if checkErr != nil {
			return fmt.Errorf("check is following: %w", checkErr)
		}
		if isFollowing {
			return nil
		}

		if err := h.followRepo.Follow(txCtx, follow); err != nil {
			return fmt.Errorf("persist follow: %w", err)
		}

		if h.userSnapshotRepo != nil {
			if err := h.userSnapshotRepo.UpdateFollowCounts(txCtx, cmd.FollowerID, cmd.FollowingID, 1); err != nil {
				return fmt.Errorf("update follow counts: %w", err)
			}
		}

		var followerName, followerAvatarURL string
		if h.userSnapshotRepo != nil {
			if u, err := h.userSnapshotRepo.GetByID(txCtx, cmd.FollowerID); err == nil && u != nil {
				followerName = u.FullName()
				followerAvatarURL = u.AvatarURL()
			}
		}

		if h.eventRecorder != nil {
			ev := &event.UserFollowedEvent{
				FollowerID:        cmd.FollowerID,
				FollowingID:       cmd.FollowingID,
				FollowedAt:        follow.CreatedAt(),
				FollowerName:      followerName,
				FollowerAvatarURL: followerAvatarURL,
			}
			if err := h.eventRecorder.RecordEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("record user followed event: %w", err)
			}
		}

		return nil
	})
}
