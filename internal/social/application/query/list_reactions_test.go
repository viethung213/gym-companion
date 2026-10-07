package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestListReactionsHandler(t *testing.T) {
	now := time.Now().UTC()
	r, _ := entity.NewReaction("r-1", "u-1", "feed-1", vo.ReactionTypeLike, now)
	interactionRepo := &queryMockInteractionRepo{reactions: []*entity.Reaction{r}}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-1": entity.NewUserSnapshot("u-1", "Liker", "http://avatar.jpg", "user", time.Time{}),
		},
	}

	handler := NewListReactionsHandler(interactionRepo, userSnapshotRepo)
	res, err := handler.Handle(context.Background(), ListReactionsQuery{
		FeedItemID: "feed-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(res.Reactions))
	}
	if res.Reactions[0].UserID != "u-1" {
		t.Errorf("expected UserID 'u-1', got '%s'", res.Reactions[0].UserID)
	}
	if res.Reactions[0].AuthorName != "Liker" {
		t.Errorf("expected AuthorName 'Liker', got '%s'", res.Reactions[0].AuthorName)
	}
	if res.Reactions[0].ReactionType != "LIKE" {
		t.Errorf("expected ReactionType 'LIKE', got '%s'", res.Reactions[0].ReactionType)
	}
	if res.TotalCount != 1 {
		t.Errorf("expected TotalCount 1, got %d", res.TotalCount)
	}
}
