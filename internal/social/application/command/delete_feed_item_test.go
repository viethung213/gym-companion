package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestDeleteFeedItemHandler(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	p, _ := aggregate.NewPostItem("f-1", "u-author", "Test post", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
	_ = feedItemRepo.Create(context.Background(), p)

	tx := &mockTxManager{}
	handler := NewDeleteFeedItemHandler(feedItemRepo, tx)

	// 1. Stranger tries to delete -> ErrUnauthorized
	err := handler.Handle(context.Background(), DeleteFeedItemCommand{
		FeedItemID: "f-1",
		UserID:     "u-stranger",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// 2. Author deletes -> Success
	err = handler.Handle(context.Background(), DeleteFeedItemCommand{
		FeedItemID: "f-1",
		UserID:     "u-author",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3. Deleting non-existent post -> ErrPostNotFound
	err = handler.Handle(context.Background(), DeleteFeedItemCommand{
		FeedItemID: "f-1",
		UserID:     "u-author",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}

	// 4. Validation: empty feed item id
	err = handler.Handle(context.Background(), DeleteFeedItemCommand{
		FeedItemID: "",
		UserID:     "u-author",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound on empty feed item id, got %v", err)
	}

	// 5. Validation: empty user id
	err = handler.Handle(context.Background(), DeleteFeedItemCommand{
		FeedItemID: "f-1",
		UserID:     "",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on empty user id, got %v", err)
	}
}
