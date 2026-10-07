package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestGetUserProfileFeedHandler(t *testing.T) {
	now := time.Now().UTC()
	p, _ := aggregate.NewPostItem("p-1", "user-target", "Profile post", nil, vo.VisibilityPublic, 1, 0, now, now)

	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{p}}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"user-target": entity.NewUserSnapshot("user-target", "Target User", "http://avatar.jpg", "user", time.Time{}),
		},
	}

	handler := NewGetUserProfileFeedHandler(feedItemRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetUserProfileFeedQuery{
		TargetUserID:  "user-target",
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Caption != "Profile post" {
		t.Errorf("expected 'Profile post', got '%s'", res.Items[0].Caption)
	}
}
