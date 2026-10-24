package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestGetSocialSummaryHandler(t *testing.T) {
	followRepo := &queryMockFollowRepo{}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{nil, nil}} // 2 items
	userRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-target": entity.NewUserSnapshotWithCounts("u-target", "Target User", "https://avatar.png", "user", 42, 17, time.Time{}),
		},
	}

	t.Run("retrieves counts directly from userSnapshotRepo", func(t *testing.T) {
		handler := NewGetSocialSummaryHandler(followRepo, feedItemRepo, userRepo)
		res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
			TargetUserID:  "u-target",
			CurrentUserID: "u-me",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.FollowerCount != 42 || res.FollowingCount != 17 || res.PostCount != 2 || !res.IsFollowing || res.FullName != "Target User" || res.AvatarURL != "https://avatar.png" || res.Role != "user" {
			t.Errorf("unexpected summary result: %+v", res)
		}
	})
}
