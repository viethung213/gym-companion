package worker

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type mockReactionBuffer struct {
	tasks []port.ReactionFlushTask
	acked []string
}

func (m *mockReactionBuffer) BufferReaction(ctx context.Context, userID, feedItemID, reactionType string, holdDuration, ttl time.Duration) error {
	return nil
}

func (m *mockReactionBuffer) GetPendingReaction(ctx context.Context, userID, feedItemID string) (string, bool, error) {
	return "", false, nil
}

func (m *mockReactionBuffer) PopMaturedReactions(ctx context.Context, now time.Time, limit int) ([]port.ReactionFlushTask, error) {
	t := m.tasks
	m.tasks = nil
	return t, nil
}

func (m *mockReactionBuffer) AckReaction(ctx context.Context, userID, feedItemID string) error {
	m.acked = append(m.acked, userID+":"+feedItemID)
	return nil
}

type mockWorkerInteractionRepo struct {
	repository.InteractionRepository
	reactions map[string]*entity.Reaction
}

func (m *mockWorkerInteractionRepo) SaveReaction(ctx context.Context, r *entity.Reaction) error {
	m.reactions[r.UserID()+":"+r.FeedItemID()] = r
	return nil
}

func (m *mockWorkerInteractionRepo) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	delete(m.reactions, userID+":"+feedItemID)
	return nil
}

func (m *mockWorkerInteractionRepo) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	return m.reactions[userID+":"+feedItemID], nil
}

type mockWorkerFeedItemRepo struct {
	repository.FeedItemRepository
	item *aggregate.FeedItem
}

func (m *mockWorkerFeedItemRepo) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return m.item, nil
}

func (m *mockWorkerFeedItemRepo) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	if m.item != nil {
		if delta > 0 {
			m.item.IncrementReaction()
		} else if delta < 0 {
			m.item.DecrementReaction()
		}
	}
	return nil
}

type mockWorkerTxManager struct{}

func (m *mockWorkerTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockWorkerEventRecorder struct {
	events []any
}

func (m *mockWorkerEventRecorder) RecordEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

func TestReactionDebounceWorker_ProcessBatch(t *testing.T) {
	feedItem, _ := aggregate.NewPostItem("f-1", "author-1", "Post", nil, vo.VisibilityPublic, 0, 0, time.Time{}, time.Time{})
	feedRepo := &mockWorkerFeedItemRepo{item: feedItem}
	interactionRepo := &mockWorkerInteractionRepo{reactions: make(map[string]*entity.Reaction)}
	tx := &mockWorkerTxManager{}
	eventRec := &mockWorkerEventRecorder{}

	buf := &mockReactionBuffer{
		tasks: []port.ReactionFlushTask{
			{UserID: "u-1", FeedItemID: "f-1", ReactionType: "LIKE"},
		},
	}

	worker := NewReactionDebounceWorker(buf, interactionRepo, feedRepo, nil, eventRec, tx, 100*time.Millisecond)

	ctx := context.Background()

	// 1. Process LIKE task
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch failed: %v", err)
	}

	if feedItem.ReactionCount() != 1 {
		t.Errorf("got reaction count %d, want 1", feedItem.ReactionCount())
	}
	if len(eventRec.events) != 1 {
		t.Errorf("got %d events recorded, want 1", len(eventRec.events))
	}
	if len(buf.acked) != 1 || buf.acked[0] != "u-1:f-1" {
		t.Errorf("got acked %v, want [u-1:f-1]", buf.acked)
	}

	// 2. Process UNLIKE task
	buf.tasks = []port.ReactionFlushTask{
		{UserID: "u-1", FeedItemID: "f-1", ReactionType: ""},
	}
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch 2 failed: %v", err)
	}

	if feedItem.ReactionCount() != 0 {
		t.Errorf("got reaction count %d, want 0", feedItem.ReactionCount())
	}
	if len(buf.acked) != 2 {
		t.Errorf("got %d acked tasks, want 2", len(buf.acked))
	}
}
