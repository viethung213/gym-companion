package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestGetFollowersHandler(t *testing.T) {
	now := time.Now().UTC()
	f1, _ := aggregate.NewFollow("f-1", "user-follower-1", "user-target", now)
	f2, _ := aggregate.NewFollow("f-2", "user-follower-2", "user-target", now)

	followRepo := &queryMockFollowRepo{
		followers: []*aggregate.Follow{f1, f2},
	}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"user-follower-1": entity.NewUserSnapshot("user-follower-1", "Follower One", "http://avatar1.jpg", "user", now),
			"user-follower-2": entity.NewUserSnapshot("user-follower-2", "Follower Two", "http://avatar2.jpg", "user", now),
		},
	}

	handler := NewGetFollowersHandler(followRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetFollowersQuery{
		UserID:        "user-target",
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Followers) != 2 {
		t.Fatalf("expected 2 followers, got %d", len(res.Followers))
	}
	if res.Followers[0].UserID != "user-follower-1" || res.Followers[0].FullName != "Follower One" {
		t.Errorf("unexpected follower 1 info: %+v", res.Followers[0])
	}
	if res.Followers[1].UserID != "user-follower-2" || res.Followers[1].FullName != "Follower Two" {
		t.Errorf("unexpected follower 2 info: %+v", res.Followers[1])
	}
}
