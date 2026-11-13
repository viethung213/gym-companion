package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/infrastructure/redis"
)

func TestFollowDebounceBuffer_Lifecycle(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	buf := redis.NewFollowDebounceBuffer(client)
	ctx := context.Background()

	// 1. Initially no pending follow
	isFollowing, exists, err := buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists || isFollowing {
		t.Errorf("got exists=%v, isFollowing=%v, want false, false", exists, isFollowing)
	}

	// 2. Buffer follow (true)
	if err := buf.BufferFollow(ctx, "u-follower", "u-following", true, 5*time.Second, 60*time.Second); err != nil {
		t.Fatalf("buffer follow failed: %v", err)
	}

	// 3. Immediately query pending
	isFollowing, exists, err = buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists || !isFollowing {
		t.Errorf("got exists=%v, isFollowing=%v, want true, true", exists, isFollowing)
	}

	// 4. Update follow to unfollow (false) before deadline expires
	if err := buf.BufferFollow(ctx, "u-follower", "u-following", false, 5*time.Second, 60*time.Second); err != nil {
		t.Fatalf("buffer unfollow failed: %v", err)
	}

	// 5. Query updated pending
	isFollowing, exists, err = buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists || isFollowing {
		t.Errorf("got exists=%v, isFollowing=%v, want true, false", exists, isFollowing)
	}

	// 6. Pop matured after 6s
	matured, err := buf.PopMaturedFollows(ctx, time.Now().Add(6*time.Second), 10)
	if err != nil {
		t.Fatalf("pop matured failed: %v", err)
	}
	if len(matured) != 1 || matured[0].IsFollowing != false {
		t.Fatalf("expected 1 task with IsFollowing=false, got %+v", matured)
	}

	// 7. Ack task
	if err := buf.AckFollow(ctx, "u-follower", "u-following"); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	// 8. Verify cleaned up
	_, exists, _ = buf.GetPendingFollow(ctx, "u-follower", "u-following")
	if exists {
		t.Errorf("expected cleaned up after ack")
	}
}

func TestFollowDebounceBuffer_ConcurrentWorkers_NoContention(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	buf := redis.NewFollowDebounceBuffer(client)
	ctx := context.Background()

	// Buffer 1 follow task
	if err := buf.BufferFollow(ctx, "u-f-race", "u-t-race", true, 5*time.Second, 60*time.Second); err != nil {
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
			tasks, err := buf.PopMaturedFollows(ctx, now, 10)
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
