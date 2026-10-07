package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type DeleteFeedItemCommand struct {
	FeedItemID string
	UserID     string
}

func (c DeleteFeedItemCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.FeedItemID) == "" {
		return derror.ErrPostNotFound
	}
	return nil
}

type DeleteFeedItemHandler struct {
	feedItemRepo repository.FeedItemRepository
	txManager    port.TransactionManager
}

func NewDeleteFeedItemHandler(
	feedItemRepo repository.FeedItemRepository,
	txManager port.TransactionManager,
) *DeleteFeedItemHandler {
	return &DeleteFeedItemHandler{
		feedItemRepo: feedItemRepo,
		txManager:    txManager,
	}
}

func (h *DeleteFeedItemHandler) Handle(ctx context.Context, cmd DeleteFeedItemCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.feedItemRepo.GetByID(txCtx, cmd.FeedItemID)
		if err != nil {
			return fmt.Errorf("get feed item: %w", err)
		}
		if item == nil {
			return derror.ErrPostNotFound
		}
		if item.UserID() != cmd.UserID {
			return derror.ErrUnauthorized
		}

		if err := h.feedItemRepo.Delete(txCtx, cmd.FeedItemID, cmd.UserID); err != nil {
			return fmt.Errorf("delete feed item: %w", err)
		}
		return nil
	})
}
