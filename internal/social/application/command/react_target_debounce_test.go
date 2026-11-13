package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type mockReactionDebounceBuffer struct {
	buffered map[string]string // key = userID:feedItemID -> reactionType
	lastHold time.Duration
	lastTTL  time.Duration
}

func newMockReactionDebounceBuffer() *mockReactionDebounceBuffer {
	return &mockReactionDebounceBuffer{
		buffered: make(map[string]string),
	}
}

func (m *mockReactionDebounceBuffer) BufferReaction(
	ctx context.Context,
	userID, feedItemID, reactionType string,
	holdDuration, ttl time.Duration,
) error {
	m.buffered[userID+":"+feedItemID] = reactionType
	m.lastHold = holdDuration
	m.lastTTL = ttl
	return nil
}

func (m *mockReactionDebounceBuffer) GetPendingReaction(
	ctx context.Context,
	userID, feedItemID string,
) (string, bool, error) {
	val, ok := m.buffered[userID+":"+feedItemID]
	return val, ok, nil
}

func (m *mockReactionDebounceBuffer) PopMaturedReactions(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]port.ReactionFlushTask, error) {
	return nil, nil
}

func (m *mockReactionDebounceBuffer) AckReaction(ctx context.Context, userID, feedItemID string) error {
	delete(m.buffered, userID+":"+feedItemID)
	return nil
}

func TestReactTargetHandler_DebounceSpam(t *testing.T) {
	feedItemRepo := &mockFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	p, _ := aggregate.NewPostItem("f-100", "u-author", "Test debounce", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
	_ = feedItemRepo.Create(context.Background(), p)

	interactionRepo := &mockInteractionRepo{
		reactions: make(map[string]*entity.Reaction),
		comments:  make(map[string]*entity.Comment),
	}
	tx := &mockTxManager{}
	pub := &mockEventPublisher{}
	uInfo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"u-spammer": entity.NewUserSnapshot("u-spammer", "Spammer User", "https://example.com/avatar.png", "user", time.Time{}),
		},
	}

	buf := newMockReactionDebounceBuffer()
	handler := NewReactTargetHandler(
		interactionRepo,
		feedItemRepo,
		uInfo,
		pub,
		tx,
		WithReactTargetDebounceBuffer(buf),
	)

	// User clicks LIKE 4 times rapidly (Spam simulation)
	steps := []struct {
		name         string
		reactionType string
		wantReaction string
	}{
		{name: "1st click LIKE (activate)", reactionType: "LIKE", wantReaction: "LIKE"},
		{name: "2nd click LIKE (toggle off)", reactionType: "LIKE", wantReaction: ""},
		{name: "3rd click LIKE (activate again)", reactionType: "LIKE", wantReaction: "LIKE"},
		{name: "4th click FIRE (switch reaction)", reactionType: "FIRE", wantReaction: "FIRE"},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			res, err := handler.Handle(context.Background(), ReactTargetCommand{
				UserID:       "u-spammer",
				FeedItemID:   "f-100",
				ReactionType: step.reactionType,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.CurrentReaction != step.wantReaction {
				t.Errorf("got %q, want %q", res.CurrentReaction, step.wantReaction)
			}
		})
	}

	// Verify that buffer received the final state FIRE
	finalPending, exists, _ := buf.GetPendingReaction(context.Background(), "u-spammer", "f-100")
	if !exists || finalPending != "FIRE" {
		t.Errorf("got pending reaction %q (exists=%v), want FIRE", finalPending, exists)
	}

	// Verify buffer parameters (5s hold duration, 60s timeout TTL)
	if buf.lastHold != 5*time.Second {
		t.Errorf("got hold duration %v, want 5s", buf.lastHold)
	}
	if buf.lastTTL != 60*time.Second {
		t.Errorf("got timeout TTL %v, want 60s", buf.lastTTL)
	}

	// In debounce mode, synchronous DB has NOT been written yet and NO event has been published yet
	if len(pub.published) != 0 {
		t.Errorf("expected 0 events published immediately during debounce, got %d", len(pub.published))
	}
}
