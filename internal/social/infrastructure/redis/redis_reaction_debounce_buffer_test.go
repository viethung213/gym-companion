package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/infrastructure/redis"
)

func TestReactionDebounceBuffer_Lifecycle(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	buf := redis.NewReactionDebounceBuffer(client)
	ctx := context.Background()

	// 1. Initially no pending reaction
	reaction, exists, err := buf.GetPendingReaction(ctx, "u-1", "f-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists || reaction != "" {
		t.Errorf("got exists=%v, reaction=%q, want false, empty", exists, reaction)
	}

	// 2. Buffer reaction LIKE with 5s hold and 60s TTL
	if err := buf.BufferReaction(ctx, "u-1", "f-1", "LIKE", 5*time.Second, 60*time.Second); err != nil {
		t.Fatalf("buffer reaction failed: %v", err)
	}

	// 3. Immediately query pending
	reaction, exists, err = buf.GetPendingReaction(ctx, "u-1", "f-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists || reaction != "LIKE" {
		t.Errorf("got exists=%v, reaction=%q, want true, LIKE", exists, reaction)
	}

	// 4. Pop matured with now (should be 0 matured because 5s has not elapsed)
	now := time.Now()
	matured, err := buf.PopMaturedReactions(ctx, now, 10)
	if err != nil {
		t.Fatalf("pop matured failed: %v", err)
	}
	if len(matured) != 0 {
		t.Errorf("expected 0 matured tasks before 5s, got %d", len(matured))
	}

	// 5. Pop matured with now + 6s (task should be matured)
	maturedFuture, err := buf.PopMaturedReactions(ctx, now.Add(6*time.Second), 10)
	if err != nil {
		t.Fatalf("pop matured future failed: %v", err)
	}
	if len(maturedFuture) != 1 {
		t.Fatalf("expected 1 matured task, got %d", len(maturedFuture))
	}
	if maturedFuture[0].UserID != "u-1" || maturedFuture[0].FeedItemID != "f-1" || maturedFuture[0].ReactionType != "LIKE" {
		t.Errorf("unexpected task payload: %+v", maturedFuture[0])
	}

	// 6. Ack task
	if err := buf.AckReaction(ctx, "u-1", "f-1"); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	// 7. Verify cleaned up
	reaction, exists, err = buf.GetPendingReaction(ctx, "u-1", "f-1")
	if err != nil || exists || reaction != "" {
		t.Errorf("expected clean state after ack, got reaction=%q, exists=%v, err=%v", reaction, exists, err)
	}
}

func TestReactionDebounceBuffer_ConcurrentWorkers_NoContention(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	buf := redis.NewReactionDebounceBuffer(client)
	ctx := context.Background()

	// Buffer 1 reaction
	if err := buf.BufferReaction(ctx, "u-race", "f-race", "LIKE", 5*time.Second, 60*time.Second); err != nil {
		t.Fatalf("buffer failed: %v", err)
	}

	// Simulate 3 backend instances competing to claim the same matured task at the same time
	numWorkers := 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalClaimed := 0

	now := time.Now().Add(6 * time.Second)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tasks, err := buf.PopMaturedReactions(ctx, now, 10)
			if err != nil {
				t.Errorf("worker pop error: %v", err)
				return
			}
			mu.Lock()
			totalClaimed += len(tasks)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Exactly 1 worker must have claimed the task! The other 2 must get 0!
	if totalClaimed != 1 {
		t.Errorf("expected exactly 1 worker to claim task, got %d claimed across %d workers", totalClaimed, numWorkers)
	}
}
