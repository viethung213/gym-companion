package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestDeleteCommentHandler(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	p, _ := aggregate.NewPostItem("f-1", "u-author", "Test post", nil, vo.VisibilityPublic, 0, 1, time.Time{}, time.Time{})
	_ = feedItemRepo.Create(context.Background(), p)

	interactionRepo := &mockInteractionRepo{
		reactions: make(map[string]*entity.Reaction),
		comments:  make(map[string]*entity.Comment),
	}
	c, _ := entity.NewComment("c-1", "u-commenter", "f-1", nil, "Great!", time.Time{}, time.Time{})
	interactionRepo.comments["c-1"] = c

	tx := &mockTxManager{}
	handler := NewDeleteCommentHandler(interactionRepo, feedItemRepo, tx)

	// 1. Stranger tries to delete -> ErrUnauthorized
	err := handler.Handle(context.Background(), DeleteCommentCommand{
		CommentID: "c-1",
		UserID:    "u-stranger",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// 2. Post owner deletes comment -> Allowed!
	err = handler.Handle(context.Background(), DeleteCommentCommand{
		CommentID: "c-1",
		UserID:    "u-author",
	})
	if err != nil {
		t.Fatalf("expected post owner deletion to succeed, got %v", err)
	}
	if p.CommentCount() != 0 {
		t.Errorf("expected comment count 0, got %d", p.CommentCount())
	}

	// 3. Deleting non-existent comment -> ErrCommentNotFound
	err = handler.Handle(context.Background(), DeleteCommentCommand{
		CommentID: "c-1",
		UserID:    "u-author",
	})
	if !errors.Is(err, derror.ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound, got %v", err)
	}

	// 4. Validation: empty comment id
	err = handler.Handle(context.Background(), DeleteCommentCommand{
		CommentID: "",
		UserID:    "u-author",
	})
	if !errors.Is(err, derror.ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound on empty comment id, got %v", err)
	}

	// 5. Validation: empty user id
	err = handler.Handle(context.Background(), DeleteCommentCommand{
		CommentID: "c-valid",
		UserID:    "",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on empty user id, got %v", err)
	}
}
