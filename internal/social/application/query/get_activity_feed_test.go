package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestGetActivityFeedHandler(t *testing.T) {
	now := time.Now().UTC()
	p, _ := aggregate.NewPostItem("p-1", "user-friend", "Great workout", nil, vo.VisibilityPublic, 2, 1, now, now)
	metrics := vo.NewWorkoutMetrics("sess-1", "Leg day", 3600, 4000.0, 5, 15)
	a, _ := aggregate.NewWorkoutActivityItem("a-1", "user-friend", "Leg day", nil, metrics, vo.VisibilityPublic, 5, 2, now.Add(time.Minute), now.Add(time.Minute))

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-friend"}}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{a, p}}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"user-friend": entity.NewUserSnapshot("user-friend", "Friend Name", "http://avatar.jpg", "user", time.Time{}),
		},
	}

	handler := NewGetActivityFeedHandler(followRepo, feedItemRepo, interactionRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items in feed, got %d", len(res.Items))
	}
	if res.Items[0].ItemType != "WORKOUT_ACTIVITY" {
		t.Errorf("expected first item to be WORKOUT_ACTIVITY, got %s", res.Items[0].ItemType)
	}
	if res.Items[0].AuthorName != "Friend Name" {
		t.Errorf("expected author name 'Friend Name', got '%s'", res.Items[0].AuthorName)
	}
}
