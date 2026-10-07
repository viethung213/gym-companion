package query

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
)

func TestGetSocialSummaryHandler(t *testing.T) {
	followRepo := &queryMockFollowRepo{}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{nil, nil}} // 2 items
	handler := NewGetSocialSummaryHandler(followRepo, feedItemRepo)

	res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
		TargetUserID:  "u-target",
		CurrentUserID: "u-me",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.FollowerCount != 10 || res.FollowingCount != 5 || res.PostCount != 2 || !res.IsFollowing {
		t.Errorf("unexpected summary result: %+v", res)
	}
}
