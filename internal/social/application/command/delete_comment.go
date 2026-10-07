package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type DeleteCommentCommand struct {
	CommentID string
	UserID    string
}

func (c DeleteCommentCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.CommentID) == "" {
		return derror.ErrCommentNotFound
	}
	return nil
}

type DeleteCommentHandler struct {
	interactionRepo repository.InteractionRepository
	feedItemRepo    repository.FeedItemRepository
	txManager       port.TransactionManager
}

func NewDeleteCommentHandler(
	interactionRepo repository.InteractionRepository,
	feedItemRepo repository.FeedItemRepository,
	txManager port.TransactionManager,
) *DeleteCommentHandler {
	return &DeleteCommentHandler{
		interactionRepo: interactionRepo,
		feedItemRepo:    feedItemRepo,
		txManager:       txManager,
	}
}

func (h *DeleteCommentHandler) Handle(ctx context.Context, cmd DeleteCommentCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		comment, err := h.interactionRepo.GetCommentByID(txCtx, cmd.CommentID)
		if err != nil {
			return fmt.Errorf("get comment: %w", err)
		}
		if comment == nil {
			return derror.ErrCommentNotFound
		}

		// Kiểm tra quyền xóa:
		// 1. Tác giả của bình luận
		// 2. Hoặc chủ bài viết chứa bình luận đó
		isCommentAuthor := comment.UserID() == cmd.UserID
		var isPostOwner bool
		if !isCommentAuthor && h.feedItemRepo != nil {
			feedItem, err := h.feedItemRepo.GetByID(txCtx, comment.FeedItemID())
			if err == nil && feedItem != nil && feedItem.UserID() == cmd.UserID {
				isPostOwner = true
			}
		}

		if !isCommentAuthor && !isPostOwner {
			return derror.ErrUnauthorized
		}

		if err := h.interactionRepo.DeleteComment(txCtx, cmd.CommentID, comment.UserID()); err != nil {
			return fmt.Errorf("delete comment: %w", err)
		}

		if err := h.feedItemRepo.UpdateCommentCount(txCtx, comment.FeedItemID(), -1); err != nil {
			return fmt.Errorf("decrement comment count: %w", err)
		}

		return nil
	})
}
