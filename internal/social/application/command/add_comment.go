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
)

type AddCommentCommand struct {
	UserID     string
	FeedItemID string
	Content    string
	ParentID   *string
}

func (c AddCommentCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.FeedItemID) == "" {
		return derror.ErrPostNotFound
	}
	if strings.TrimSpace(c.Content) == "" {
		return derror.ErrEmptyContent
	}
	return nil
}

type AddCommentResult struct {
	Comment *entity.Comment
}

type AddCommentHandler struct {
	interactionRepo  repository.InteractionRepository
	feedItemRepo     repository.FeedItemRepository
	userSnapshotRepo repository.UserSnapshotRepository
	eventPub         port.EventPublisher
	txManager        port.TransactionManager
}

func NewAddCommentHandler(
	interactionRepo repository.InteractionRepository,
	feedItemRepo repository.FeedItemRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *AddCommentHandler {
	return &AddCommentHandler{
		interactionRepo:  interactionRepo,
		feedItemRepo:     feedItemRepo,
		userSnapshotRepo: userSnapshotRepo,
		eventPub:         eventPub,
		txManager:        txManager,
	}
}

func (h *AddCommentHandler) Handle(ctx context.Context, cmd AddCommentCommand) (*AddCommentResult, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	var parentID *string
	if cmd.ParentID != nil && strings.TrimSpace(*cmd.ParentID) != "" {
		trimmed := strings.TrimSpace(*cmd.ParentID)
		parentID = &trimmed
	}

	now := time.Now().UTC()
	comment, createErr := entity.NewComment(
		uuid.New().String(),
		cmd.UserID,
		cmd.FeedItemID,
		parentID,
		cmd.Content,
		now,
		now,
	)
	if createErr != nil {
		return nil, fmt.Errorf("create comment entity: %w", createErr)
	}

	txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// 1. Kiểm tra bài viết tồn tại (Fail-fast)
		feedItem, err := h.feedItemRepo.GetByID(txCtx, cmd.FeedItemID)
		if err != nil {
			return fmt.Errorf("get target feed item: %w", err)
		}
		if feedItem == nil {
			return derror.ErrPostNotFound
		}

		// 2. Kiểm tra người dùng tồn tại và lấy thông tin hiển thị
		var actorName, actorAvatarURL string
		if h.userSnapshotRepo != nil {
			user, err := h.userSnapshotRepo.GetByID(txCtx, cmd.UserID)
			if err != nil {
				return fmt.Errorf("get user snapshot: %w", err)
			}
			if user == nil {
				return derror.ErrUnauthorized
			}
			actorName = user.FullName()
			actorAvatarURL = user.AvatarURL()
		}

		// 3. Kiểm tra comment cha tồn tại và thuộc cùng bài viết nếu là reply
		var parentCommentOwnerID *string
		if parentID != nil {
			parentComment, err := h.interactionRepo.GetCommentByID(txCtx, *parentID)
			if err != nil {
				return fmt.Errorf("get parent comment: %w", err)
			}
			if parentComment == nil || parentComment.FeedItemID() != cmd.FeedItemID {
				return derror.ErrCommentNotFound
			}
			parentOwner := parentComment.UserID()
			parentCommentOwnerID = &parentOwner
		}

		// 4. Thêm bình luận
		if err := h.interactionRepo.AddComment(txCtx, comment); err != nil {
			return fmt.Errorf("add comment: %w", err)
		}

		// 5. Tăng số lượng bình luận trên bài viết
		if err := h.feedItemRepo.UpdateCommentCount(txCtx, cmd.FeedItemID, 1); err != nil {
			return fmt.Errorf("increment feed item comment count: %w", err)
		}

		// 6. Phát Event bình luận
		if h.eventPub != nil {
			ev := &event.PostCommentedEvent{
				CommentID:            comment.ID(),
				FeedItemID:           cmd.FeedItemID,
				PostOwnerID:          feedItem.UserID(),
				ParentCommentOwnerID: parentCommentOwnerID,
				UserID:               cmd.UserID,
				UserName:             actorName,
				UserAvatarURL:        actorAvatarURL,
				Content:              comment.Content(),
				ParentID:             comment.ParentID(),
				CommentedAt:          comment.CreatedAt(),
			}
			if err := h.eventPub.PublishEvents(txCtx, []any{ev}); err != nil {
				return fmt.Errorf("publish post commented event: %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return &AddCommentResult{Comment: comment}, nil
}
