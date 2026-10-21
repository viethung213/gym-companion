package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestUnfollowUserHandler(t *testing.T) {
	followRepo := &mockFollowRepo{follows: make(map[string]*aggregate.Follow)}
	f, _ := aggregate.NewFollow("f-1", "u-1", "u-2", time.Now().UTC())
	_ = followRepo.Follow(context.Background(), f)

	userRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-1": entity.NewUserSnapshotWithCounts("u-1", "Follower", "", "user", 0, 1, time.Time{}),
			"u-2": entity.NewUserSnapshotWithCounts("u-2", "Target", "", "user", 1, 0, time.Time{}),
		},
	}
	tx := &mockTxManager{}
	handler := NewUnfollowUserHandler(followRepo, userRepo, tx)

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

		u1, _ := userRepo.GetByID(context.Background(), "u-1")
		u2, _ := userRepo.GetByID(context.Background(), "u-2")
		if u1.FollowingCount() != 0 || u2.FollowerCount() != 0 {
			t.Errorf("expected counts to be 0 after unfollow, got u1=%d, u2=%d", u1.FollowingCount(), u2.FollowerCount())
		}

		// Idempotent unfollow
		err = handler.Handle(context.Background(), UnfollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "u-2",
		})
		if err != nil {
			t.Fatalf("unexpected error on idempotent unfollow: %v", err)
		}
		u1, _ = userRepo.GetByID(context.Background(), "u-1")
		if u1.FollowingCount() != 0 {
			t.Errorf("expected counts to remain 0, got u1=%d", u1.FollowingCount())
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
