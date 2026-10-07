package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestGetFollowingHandler(t *testing.T) {
	now := time.Now().UTC()
	f1, _ := aggregate.NewFollow("f-1", "user-me", "user-following-1", now)
	f2, _ := aggregate.NewFollow("f-2", "user-me", "user-following-2", now)

	followRepo := &queryMockFollowRepo{
		following: []*aggregate.Follow{f1, f2},
	}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"user-following-1": entity.NewUserSnapshot("user-following-1", "Following One", "http://avatar1.jpg", "user", now),
			"user-following-2": entity.NewUserSnapshot("user-following-2", "Following Two", "http://avatar2.jpg", "user", now),
		},
	}

	handler := NewGetFollowingHandler(followRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetFollowingQuery{
		UserID:        "user-me",
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Following) != 2 {
		t.Fatalf("expected 2 following users, got %d", len(res.Following))
	}
	if res.Following[0].UserID != "user-following-1" || res.Following[0].FullName != "Following One" {
		t.Errorf("unexpected following 1 info: %+v", res.Following[0])
	}
	if res.Following[1].UserID != "user-following-2" || res.Following[1].FullName != "Following Two" {
		t.Errorf("unexpected following 2 info: %+v", res.Following[1])
	}
}
