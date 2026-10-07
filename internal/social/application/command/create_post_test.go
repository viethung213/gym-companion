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

func TestCreatePostHandler(t *testing.T) {
	t.Run("success with caption and media", func(t *testing.T) {
		feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
		tx := &mockTxManager{}
		pub := &mockEventPublisher{}
		handler := NewCreatePostHandler(feedItemRepo, pub, tx)

		res, err := handler.Handle(context.Background(), CreatePostCommand{
			UserID:     "u-1",
			Caption:    "Hello gym world!",
			MediaURLs:  []string{"http://img.com/1.png"},
			Visibility: "PUBLIC",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Item == nil || res.Item.Caption() != "Hello gym world!" {
			t.Errorf("unexpected post result")
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 event published, got %d", len(pub.published))
		}
	})

	t.Run("success with followers and author snapshot populates PostCreatedEvent", func(t *testing.T) {
		feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
		tx := &mockTxManager{}
		pub := &mockEventPublisher{}
		uInfo := &mockUserSnapshotRepo{
			users: map[string]*entity.UserSnapshot{
				"u-author": entity.NewUserSnapshot("u-author", "Gym Bro", "https://img.com/avatar.png", "user", time.Time{}),
			},
		}
		f1, _ := aggregate.NewFollow("fol-1", "f-user-1", "u-author", time.Time{})
		f2, _ := aggregate.NewFollow("fol-2", "f-user-2", "u-author", time.Time{})
		fRepo := &mockFollowRepo{
			follows: map[string]*aggregate.Follow{
				"fol-1": f1,
				"fol-2": f2,
			},
		}

		handler := NewCreatePostHandler(
			feedItemRepo, pub, tx,
			WithCreatePostFollowRepo(fRepo),
			WithCreatePostUserSnapshotRepo(uInfo),
		)

		res, err := handler.Handle(context.Background(), CreatePostCommand{
			UserID:     "u-author",
			Caption:    "New PR!",
			Visibility: "PUBLIC",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Item == nil {
			t.Fatal("expected created item")
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 event published, got %d", len(pub.published))
		}
		ev, ok := pub.published[0].(*event.PostCreatedEvent)
		if !ok {
			t.Fatalf("expected *event.PostCreatedEvent, got %T", pub.published[0])
		}
		if ev.UserName != "Gym Bro" || len(ev.FollowerUserIDs) != 2 {
			t.Errorf("unexpected event content: %+v", ev)
		}
	})

	t.Run("success with only media and no caption", func(t *testing.T) {
		feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
		tx := &mockTxManager{}
		pub := &mockEventPublisher{}
		handler := NewCreatePostHandler(feedItemRepo, pub, tx)

		res, err := handler.Handle(context.Background(), CreatePostCommand{
			UserID:    "u-1",
			MediaURLs: []string{"http://img.com/only-img.png"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Item == nil {
			t.Errorf("expected post created")
		}
	})

	t.Run("validation error empty user", func(t *testing.T) {
		handler := NewCreatePostHandler(nil, nil, nil)
		_, err := handler.Handle(context.Background(), CreatePostCommand{
			UserID:  "",
			Caption: "Should fail",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Fatalf("got err %v, want ErrUnauthorized", err)
		}
	})

	t.Run("validation error empty content and empty media", func(t *testing.T) {
		handler := NewCreatePostHandler(nil, nil, nil)
		_, err := handler.Handle(context.Background(), CreatePostCommand{
			UserID:    "u-1",
			Caption:   "",
			MediaURLs: nil,
		})
		if !errors.Is(err, derror.ErrEmptyContent) {
			t.Fatalf("got err %v, want ErrEmptyContent", err)
		}
	})
}
