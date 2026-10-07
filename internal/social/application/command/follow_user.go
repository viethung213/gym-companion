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
	eventPub         port.EventPublisher
	txManager        port.TransactionManager
}

func NewFollowUserHandler(
	followRepo repository.FollowRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *FollowUserHandler {
	return &FollowUserHandler{
		followRepo:       followRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventPub:         eventPub,
		txManager:        txManager,
	}
}

func (h *FollowUserHandler) Handle(ctx context.Context, cmd FollowUserCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

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
		if err := h.followRepo.Follow(txCtx, follow); err != nil {
			return fmt.Errorf("persist follow: %w", err)
		}

		var followerName, followerAvatarURL string
		if h.userSnapshotRepo != nil {
			if u, err := h.userSnapshotRepo.GetByID(txCtx, cmd.FollowerID); err == nil && u != nil {
				followerName = u.FullName()
				followerAvatarURL = u.AvatarURL()
			}
		}

		if h.eventPub != nil {
			ev := &event.UserFollowedEvent{
				FollowerID:        cmd.FollowerID,
				FollowingID:       cmd.FollowingID,
				FollowedAt:        follow.CreatedAt(),
				FollowerName:      followerName,
				FollowerAvatarURL: followerAvatarURL,
			}
			if err := h.eventPub.PublishEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("publish user followed event: %w", err)
			}
		}

		return nil
	})
}
