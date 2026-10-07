package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestListCommentsHandler(t *testing.T) {
	now := time.Now().UTC()
	c, _ := entity.NewComment("c-1", "u-1", "feed-1", nil, "Awesome!", now, now)
	interactionRepo := &queryMockInteractionRepo{comments: []*entity.Comment{c}}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-1": entity.NewUserSnapshot("u-1", "Commenter", "http://avatar.jpg", "user", time.Time{}),
		},
	}

	handler := NewListCommentsHandler(interactionRepo, userSnapshotRepo)
	res, err := handler.Handle(context.Background(), ListCommentsQuery{
		FeedItemID: "feed-1",
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(res.Comments))
	}
	if res.Comments[0].Content != "Awesome!" {
		t.Errorf("expected content 'Awesome!', got '%s'", res.Comments[0].Content)
	}
	if res.Comments[0].FeedItemID != "feed-1" {
		t.Errorf("expected FeedItemID 'feed-1', got '%s'", res.Comments[0].FeedItemID)
	}
}
