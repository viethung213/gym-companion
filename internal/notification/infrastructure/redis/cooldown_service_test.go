package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	infraRedis "github.com/viethung213/gym-companion/internal/notification/infrastructure/redis"
)

func TestRedisPushCooldownService(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	svc := infraRedis.NewPushCooldownService(client)
	ctx := context.Background()

	feedID := "feed-100"
	userID := "user-200"
	ttl := 30 * time.Second

	// 1. First event -> lock acquired (returns true)
	allowed, err := svc.CheckAndLock(ctx, feedID, userID, ttl)
	if err != nil {
		t.Fatalf("expected no error on first lock, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected first event to be allowed, got false")
	}

	// 2. Second event during cooldown -> rejected (returns false)
	allowed2, err := svc.CheckAndLock(ctx, feedID, userID, ttl)
	if err != nil {
		t.Fatalf("expected no error on second lock check, got %v", err)
	}
	if allowed2 {
		t.Fatalf("expected second event to be suppressed by cooldown, got true")
	}

	// 3. Self-healing release lock (e.g. FCM failed) -> key is removed
	if err := svc.ReleaseLock(ctx, feedID, userID); err != nil {
		t.Fatalf("expected ReleaseLock to succeed, got %v", err)
	}

	// 4. Immediately after release -> lock can be acquired again
	allowed3, err := svc.CheckAndLock(ctx, feedID, userID, ttl)
	if err != nil {
		t.Fatalf("expected no error after release lock, got %v", err)
	}
	if !allowed3 {
		t.Fatalf("expected event after release to be allowed, got false")
	}

	// 5. Fast-forward miniredis time by ttl -> key expires automatically
	mr.FastForward(ttl + time.Second)
	allowed4, err := svc.CheckAndLock(ctx, feedID, userID, ttl)
	if err != nil {
		t.Fatalf("expected no error after ttl expiry, got %v", err)
	}
	if !allowed4 {
		t.Fatalf("expected event after ttl expiry to be allowed, got false")
	}
}

func TestRedisPushCooldownService_Batch(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	svc := infraRedis.NewPushCooldownService(client)
	ctx := context.Background()

	feedID := "feed-batch-1"
	recipients := []string{"user-1", "user-2", "user-3", "user-4"}
	ttl := 30 * time.Second

	// 1. First batch -> all 4 acquire lock
	allowed, err := svc.CheckAndLockBatch(ctx, feedID, recipients, ttl)
	if err != nil {
		t.Fatalf("expected no error on first batch lock, got %v", err)
	}
	if len(allowed) != 4 {
		t.Fatalf("expected 4 allowed recipients on first batch, got %d (%v)", len(allowed), allowed)
	}

	// 2. Immediate second batch -> all 4 suppressed (returns empty slice)
	allowed2, err := svc.CheckAndLockBatch(ctx, feedID, recipients, ttl)
	if err != nil {
		t.Fatalf("expected no error on second batch lock check, got %v", err)
	}
	if len(allowed2) != 0 {
		t.Fatalf("expected 0 allowed recipients on second batch during cooldown, got %d (%v)", len(allowed2), allowed2)
	}

	// 3. Partial release lock: release user-1 and user-3
	if err := svc.ReleaseLockBatch(ctx, feedID, []string{"user-1", "user-3"}); err != nil {
		t.Fatalf("expected ReleaseLockBatch to succeed, got %v", err)
	}

	// 4. Third batch -> only user-1 and user-3 are allowed, user-2 and user-4 still suppressed
	allowed3, err := svc.CheckAndLockBatch(ctx, feedID, recipients, ttl)
	if err != nil {
		t.Fatalf("expected no error on third batch lock check, got %v", err)
	}
	if len(allowed3) != 2 {
		t.Fatalf("expected 2 allowed recipients (user-1 and user-3), got %d (%v)", len(allowed3), allowed3)
	}
	hasUser1 := allowed3[0] == "user-1" || allowed3[1] == "user-1"
	hasUser3 := allowed3[0] == "user-3" || allowed3[1] == "user-3"
	if !hasUser1 || !hasUser3 {
		t.Fatalf("expected user-1 and user-3 to be allowed, got %v", allowed3)
	}

	// 5. Fast-forward TTL -> all 4 are unlocked
	mr.FastForward(ttl + time.Second)
	allowed4, err := svc.CheckAndLockBatch(ctx, feedID, recipients, ttl)
	if err != nil {
		t.Fatalf("expected no error after TTL expiry, got %v", err)
	}
	if len(allowed4) != 4 {
		t.Fatalf("expected 4 allowed recipients after TTL expiry, got %d (%v)", len(allowed4), allowed4)
	}
}

func TestRedisPushCooldownService_NilClientFailsOpen(t *testing.T) {
	svc := infraRedis.NewPushCooldownService(nil)
	ctx := context.Background()

	allowed, err := svc.CheckAndLock(ctx, "feed-1", "user-1", time.Minute)
	if err != nil || !allowed {
		t.Fatalf("expected nil client to fail open (true, nil), got (%v, %v)", allowed, err)
	}

	err = svc.ReleaseLock(ctx, "feed-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil client release to return nil, got %v", err)
	}

	// Batch with nil client also fails open
	batchAllowed, err := svc.CheckAndLockBatch(ctx, "feed-1", []string{"u1", "u2"}, time.Minute)
	if err != nil {
		t.Fatalf("expected nil client batch check to return nil error, got %v", err)
	}
	if len(batchAllowed) != 2 {
		t.Fatalf("expected nil client to fail open returning all recipients, got %v", batchAllowed)
	}

	err = svc.ReleaseLockBatch(ctx, "feed-1", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("expected nil client ReleaseLockBatch to return nil, got %v", err)
	}
}
