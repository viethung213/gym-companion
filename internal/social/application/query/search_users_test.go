package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestSearchUsersHandler(t *testing.T) {
	userRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-1": entity.NewUserSnapshot("u-1", "Alice Athlete", "https://avatar.com/1.png", "user", time.Now()),
			"u-2": entity.NewUserSnapshot("u-2", "Gymshark Store", "https://avatar.com/2.png", "brand", time.Now()),
			"u-3": entity.NewUserSnapshot("u-3", "Root Admin", "https://avatar.com/3.png", "admin", time.Now()),
		},
	}
	followRepo := &queryMockFollowRepo{
		followingIDs: []string{"u-2"},
	}

	handler := NewSearchUsersHandler(userRepo, followRepo)

	t.Run("search all discoverable users with is_following check", func(t *testing.T) {
		res, err := handler.Handle(context.Background(), SearchUsersQuery{
			CurrentUserID: "u-1",
			PageSize:      10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Users) != 2 {
			t.Fatalf("expected 2 users (excluding admin), got %d", len(res.Users))
		}

		for _, u := range res.Users {
			if u.Role == "admin" {
				t.Errorf("admin user should not be discoverable")
			}
			if u.UserID == "u-2" && !u.IsFollowing {
				t.Errorf("expected u-2 to have is_following = true")
			}
			if u.UserID == "u-1" && u.IsFollowing {
				t.Errorf("current user should not follow oneself")
			}
		}
	})

	t.Run("filter by brand", func(t *testing.T) {
		res, err := handler.Handle(context.Background(), SearchUsersQuery{
			Role:     "brand",
			PageSize: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Users) != 1 {
			t.Fatalf("expected 1 brand, got %d", len(res.Users))
		}
		if res.Users[0].UserID != "u-2" || res.Users[0].Role != "brand" {
			t.Errorf("unexpected brand result: %+v", res.Users[0])
		}
	})

	t.Run("filter by admin returns empty", func(t *testing.T) {
		res, err := handler.Handle(context.Background(), SearchUsersQuery{
			Role: "admin",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Users) != 0 || res.TotalCount != 0 {
			t.Errorf("expected empty results for admin role, got %d", len(res.Users))
		}
	})
}
