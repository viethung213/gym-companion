package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type ReactTargetCommand struct {
	UserID       string
	FeedItemID   string
	ReactionType string
}

func (c ReactTargetCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.FeedItemID) == "" {
		return derror.ErrPostNotFound
	}
	if _, err := vo.NewReactionType(c.ReactionType); err != nil {
		return fmt.Errorf("invalid reaction type: %w", err)
	}
	return nil
}

type ReactTargetResult struct {
	CurrentReaction string
}

type ReactTargetHandler struct {
	interactionRepo  repository.InteractionRepository
	feedItemRepo     repository.FeedItemRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventPub         port.EventPublisher
	txManager        port.TransactionManager
}

func NewReactTargetHandler(
	interactionRepo repository.InteractionRepository,
	feedItemRepo repository.FeedItemRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *ReactTargetHandler {
	return &ReactTargetHandler{
		interactionRepo:  interactionRepo,
		feedItemRepo:     feedItemRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventPub:         eventPub,
		txManager:        txManager,
	}
}

func (h *ReactTargetHandler) Handle(ctx context.Context, cmd ReactTargetCommand) (*ReactTargetResult, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	rt, _ := vo.NewReactionType(cmd.ReactionType)
	var currentReaction string

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// 1. Kiểm tra bài viết tồn tại (Fail-fast)
		feedItem, feedErr := h.feedItemRepo.GetByID(txCtx, cmd.FeedItemID)
		if feedErr != nil {
			return fmt.Errorf("get target feed item: %w", feedErr)
		}
		if feedItem == nil {
			return derror.ErrPostNotFound
		}

		// 2. Kiểm tra người dùng tồn tại và lấy thông tin hiển thị
		var actorName, actorAvatarURL string
		if h.userSnapshotRepo != nil {
			user, userErr := h.userSnapshotRepo.GetByID(txCtx, cmd.UserID)
			if userErr != nil {
				return fmt.Errorf("get user snapshot: %w", userErr)
			}
			if user == nil {
				return derror.ErrUnauthorized
			}
			actorName = user.FullName()
			actorAvatarURL = user.AvatarURL()
		}

		// 3. Kiểm tra reaction đã có của user
		existing, getErr := h.interactionRepo.GetReaction(txCtx, cmd.UserID, cmd.FeedItemID)
		if getErr != nil {
			return fmt.Errorf("check existing reaction: %w", getErr)
		}

		if existing != nil {
			if existing.ReactionType() == rt {
				// Toggle OFF: xóa reaction và giảm counter
				if delErr := h.interactionRepo.DeleteReaction(txCtx, cmd.UserID, cmd.FeedItemID); delErr != nil {
					return fmt.Errorf("delete reaction: %w", delErr)
				}
				if decrErr := h.feedItemRepo.UpdateReactionCount(txCtx, cmd.FeedItemID, -1); decrErr != nil {
					return fmt.Errorf("decrement counter: %w", decrErr)
				}
				currentReaction = ""
				return nil
			}

			// Đổi reaction type khác: cập nhật, counter giữ nguyên
			existing.ChangeReactionType(rt)
			if err := h.interactionRepo.SaveReaction(txCtx, existing); err != nil {
				return fmt.Errorf("update reaction type: %w", err)
			}
			currentReaction = string(rt)
			return nil
		}

		// Thêm mới reaction
		newReaction, err := entity.NewReaction(
			uuid.New().String(),
			cmd.UserID,
			cmd.FeedItemID,
			rt,
			time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("create reaction entity: %w", err)
		}
		if err := h.interactionRepo.SaveReaction(txCtx, newReaction); err != nil {
			return fmt.Errorf("save reaction: %w", err)
		}
		if err := h.feedItemRepo.UpdateReactionCount(txCtx, cmd.FeedItemID, 1); err != nil {
			return fmt.Errorf("increment counter: %w", err)
		}
		currentReaction = string(rt)

		if currentReaction != "" && h.eventPub != nil {
			ev := &event.PostReactedEvent{
				FeedItemID:    cmd.FeedItemID,
				PostOwnerID:   feedItem.UserID(),
				UserID:        cmd.UserID,
				UserName:      actorName,
				UserAvatarURL: actorAvatarURL,
				ReactionType:  rt,
				ReactedAt:     time.Now().UTC(),
			}
			if err := h.eventPub.PublishEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("publish post reacted event: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ReactTargetResult{CurrentReaction: currentReaction}, nil
}
