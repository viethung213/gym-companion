package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/event"
)

func TestFollowUserHandler(t *testing.T) {
	followRepo := &mockFollowRepo{follows: make(map[string]*aggregate.Follow)}
	userRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-1": entity.NewUserSnapshot("u-1", "John Doe", "https://example.com/avatar.png", "user", time.Time{}),
		},
	}
	tx := &mockTxManager{}
	pub := &mockEventPublisher{}
	handler := NewFollowUserHandler(followRepo, userRepo, pub, tx)

	t.Run("valid follow", func(t *testing.T) {
		err := handler.Handle(context.Background(), FollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "u-2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		isFollowing, _ := followRepo.IsFollowing(context.Background(), "u-1", "u-2")
		if !isFollowing {
			t.Errorf("expected isFollowing to be true")
		}

		if len(pub.published) != 1 {
			t.Fatalf("expected 1 event published, got %d", len(pub.published))
		}
		ev, ok := pub.published[0].(*event.UserFollowedEvent)
		if !ok {
			t.Fatalf("expected *event.UserFollowedEvent, got %T", pub.published[0])
		}
		if ev.FollowerName != "John Doe" || ev.FollowerAvatarURL != "https://example.com/avatar.png" {
			t.Errorf("expected follower info in event, got name=%q avatar=%q", ev.FollowerName, ev.FollowerAvatarURL)
		}
	})

	t.Run("self follow rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), FollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "u-1",
		})
		if !errors.Is(err, derror.ErrSelfFollow) {
			t.Errorf("expected ErrSelfFollow, got %v", err)
		}
	})

	t.Run("empty follower id rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), FollowUserCommand{
			FollowerID:  "",
			FollowingID: "u-2",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty following id rejected", func(t *testing.T) {
		err := handler.Handle(context.Background(), FollowUserCommand{
			FollowerID:  "u-1",
			FollowingID: "",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}
