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
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestReactTargetHandler_Toggle(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	p, _ := aggregate.NewPostItem("f-1", "u-author", "Test post", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
	_ = feedItemRepo.Create(context.Background(), p)

	interactionRepo := &mockInteractionRepo{
		reactions: make(map[string]*entity.Reaction),
		comments:  make(map[string]*entity.Comment),
	}
	tx := &mockTxManager{}
	pub := &mockEventPublisher{}
	uInfo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-viewer": entity.NewUserSnapshot("u-viewer", "Viewer User", "https://example.com/viewer.png", "user", time.Time{}),
		},
	}

	handler := NewReactTargetHandler(interactionRepo, feedItemRepo, uInfo, pub, tx)

	// Step 1: User reacts LIKE
	res1, err := handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "f-1",
		ReactionType: "LIKE",
	})
	if err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	if res1.CurrentReaction != "LIKE" {
		t.Errorf("expected reaction LIKE, got %s", res1.CurrentReaction)
	}
	if p.ReactionCount() != 1 {
		t.Errorf("expected post reaction count 1, got %d", p.ReactionCount())
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 event published on reaction, got %d", len(pub.published))
	}
	reactEv, ok := pub.published[0].(*event.PostReactedEvent)
	if !ok {
		t.Fatalf("expected *event.PostReactedEvent, got %T", pub.published[0])
	}
	if reactEv.PostOwnerID != "u-author" || reactEv.UserName != "Viewer User" || reactEv.ReactionType != "LIKE" {
		t.Errorf("unexpected event content: %+v", reactEv)
	}

	// Step 2: User reacts LIKE again -> Toggle OFF
	res2, err := handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "f-1",
		ReactionType: "LIKE",
	})
	if err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}
	if res2.CurrentReaction != "" {
		t.Errorf("expected reaction cleared, got %s", res2.CurrentReaction)
	}
	if p.ReactionCount() != 0 {
		t.Errorf("expected post reaction count 0, got %d", p.ReactionCount())
	}
	// Toggle off should not emit an event
	if len(pub.published) != 1 {
		t.Fatalf("expected still 1 event after toggle off, got %d", len(pub.published))
	}

	// Test validation: missing feed item
	_, err = handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "non-existent-feed",
		ReactionType: "LIKE",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound on missing feed item, got %v", err)
	}

	// Test validation: empty user
	_, err = handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "",
		FeedItemID:   "f-1",
		ReactionType: "LIKE",
	})
	if !errors.Is(err, derror.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on empty user, got %v", err)
	}

	// Test validation: empty feed item id
	_, err = handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "",
		ReactionType: "LIKE",
	})
	if !errors.Is(err, derror.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound on empty feed item, got %v", err)
	}

	// Test validation: invalid reaction type
	_, err = handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "f-1",
		ReactionType: "INVALID_REACTION",
	})
	if err == nil {
		t.Errorf("expected error on invalid reaction type, got nil")
	}

	// Test reaction type switch (from FIRE to MUSCLE)
	_, _ = handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "f-1",
		ReactionType: "FIRE",
	})
	resSwitch, err := handler.Handle(context.Background(), ReactTargetCommand{
		UserID:       "u-viewer",
		FeedItemID:   "f-1",
		ReactionType: "MUSCLE",
	})
	if err != nil {
		t.Fatalf("reaction switch failed: %v", err)
	}
	if resSwitch.CurrentReaction != "MUSCLE" {
		t.Errorf("expected reaction MUSCLE after switch, got %s", resSwitch.CurrentReaction)
	}
}
