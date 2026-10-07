package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
)

func TestUnfollowUserHandler(t *testing.T) {
	followRepo := &mockFollowRepo{follows: make(map[string]*aggregate.Follow)}
	f, _ := aggregate.NewFollow("f-1", "u-1", "u-2", time.Now().UTC())
	_ = followRepo.Follow(context.Background(), f)

	tx := &mockTxManager{}
	handler := NewUnfollowUserHandler(followRepo, tx)

	t.Run("valid unfollow", func(t *testing.T) {
		err := handler.Handle(context.Background(), UnfollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "u-2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		isFollowing, _ := followRepo.IsFollowing(context.Background(), "u-1", "u-2")
		if isFollowing {
			t.Errorf("expected isFollowing to be false after unfollow")
		}
	})

	t.Run("self unfollow rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), UnfollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "u-1",
		})
		if !errors.Is(err, derror.ErrSelfFollow) {
			t.Errorf("expected ErrSelfFollow, got %v", err)
		}
	})

	t.Run("empty follower id rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), UnfollowUserCommand{
			FollowerID:  "",
			FollowingID: "u-2",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty following id rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), UnfollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}
