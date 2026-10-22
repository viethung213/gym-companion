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
			"u-target": entity.NewUserSnapshotWithCounts("u-target", "Target User", "", "user", 42, 17, time.Time{}),
			"u-brand":  entity.NewUserSnapshotWithCounts("u-brand", "Gym Brand", "https://img.com/avatar.jpg", "brand", 100, 5, time.Time{}),
		},
	}

	t.Run("retrieves counts and identity from userSnapshotRepo", func(t *testing.T) {
		handler := NewGetSocialSummaryHandler(followRepo, feedItemRepo, userRepo)
		res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
			TargetUserID:  "u-target",
			CurrentUserID: "u-me",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, want := res.FullName, "Target User"; got != want {
			t.Errorf("got FullName %q, want %q", got, want)
		}
		if got, want := res.AvatarURL, entity.DefaultAvatarURL; got != want {
			t.Errorf("got AvatarURL %q, want %q", got, want)
		}
		if got, want := res.Role, "user"; got != want {
			t.Errorf("got Role %q, want %q", got, want)
		}
		if res.FollowerCount != 42 || res.FollowingCount != 17 || res.PostCount != 2 || !res.IsFollowing {
			t.Errorf("unexpected summary result: %+v", res)
		}
	})

	t.Run("user with zero posts retains identity and custom avatar", func(t *testing.T) {
		emptyFeedRepo := &queryMockFeedItemRepo{items: nil}
		handler := NewGetSocialSummaryHandler(followRepo, emptyFeedRepo, userRepo)
		res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
			TargetUserID:  "u-brand",
			CurrentUserID: "u-me",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, want := res.FullName, "Gym Brand"; got != want {
			t.Errorf("got FullName %q, want %q", got, want)
		}
		if got, want := res.AvatarURL, "https://img.com/avatar.jpg"; got != want {
			t.Errorf("got AvatarURL %q, want %q", got, want)
		}
		if got, want := res.Role, "brand"; got != want {
			t.Errorf("got Role %q, want %q", got, want)
		}
		if got, want := res.PostCount, int32(0); got != want {
			t.Errorf("got PostCount %d, want %d", got, want)
		}
		if res.FollowerCount != 100 || res.FollowingCount != 5 {
			t.Errorf("unexpected counts: %+v", res)
		}
	})

	t.Run("fallback when user snapshot is missing", func(t *testing.T) {
		handler := NewGetSocialSummaryHandler(followRepo, feedItemRepo, userRepo)
		res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
			TargetUserID:  "u-non-existent",
			CurrentUserID: "u-me",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, want := res.FullName, "User"; got != want {
			t.Errorf("got FullName %q, want %q", got, want)
		}
		if got, want := res.AvatarURL, entity.DefaultAvatarURL; got != want {
			t.Errorf("got AvatarURL %q, want %q", got, want)
		}
		if got, want := res.Role, "user"; got != want {
			t.Errorf("got Role %q, want %q", got, want)
		}
	})

	t.Run("self summary sets isFollowing to false", func(t *testing.T) {
		handler := NewGetSocialSummaryHandler(followRepo, feedItemRepo, userRepo)
		res, err := handler.Handle(context.Background(), GetSocialSummaryQuery{
			TargetUserID:  "u-target",
			CurrentUserID: "u-target",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.IsFollowing {
			t.Errorf("expected isFollowing = false when viewing self summary")
		}
	})
}
