package aggregate

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
)

func TestNewFollow(t *testing.T) {
	t.Run("valid follow", func(t *testing.T) {
		f, err := NewFollow("f-1", "user-a", "user-b", time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ID() != "f-1" || f.FollowerID() != "user-a" || f.FollowingID() != "user-b" {
			t.Errorf("unexpected follow values")
		}
		if f.CreatedAt().IsZero() {
			t.Errorf("expected non-zero created at")
		}
	})

	t.Run("self follow rejected", func(t *testing.T) {
		_, err := NewFollow("f-1", "user-a", "user-a", time.Time{})
		if err != derror.ErrSelfFollow {
			t.Errorf("expected ErrSelfFollow, got %v", err)
		}
	})

	t.Run("empty follower or following rejected", func(t *testing.T) {
		if _, err := NewFollow("f-1", "", "user-b", time.Time{}); err != derror.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for empty follower, got %v", err)
		}
		if _, err := NewFollow("f-1", "user-a", "", time.Time{}); err != derror.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for empty following, got %v", err)
		}
	})
}
