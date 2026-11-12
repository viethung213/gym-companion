package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/infrastructure/redis"
)

func TestTrendingPoolCache_SaveAndGet(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cache := redis.NewTrendingPoolCache(client)
	ctx := context.Background()

	// Empty trending pool initially
	ids, err := cache.GetTrendingPool(ctx)
	if err != nil {
		t.Fatalf("GetTrendingPool unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 ids, got %d", len(ids))
	}

	// Save trending pool
	trendingItems := []string{"hot-1", "hot-2", "hot-3"}
	if err := cache.SaveTrendingPool(ctx, trendingItems); err != nil {
		t.Fatalf("SaveTrendingPool error: %v", err)
	}

	ids, err = cache.GetTrendingPool(ctx)
	if err != nil {
		t.Fatalf("GetTrendingPool error: %v", err)
	}
	if len(ids) != 3 || ids[0] != "hot-1" || ids[1] != "hot-2" || ids[2] != "hot-3" {
		t.Fatalf("expected [hot-1, hot-2, hot-3], got %v", ids)
	}

	// Save empty trending pool should delete key
	if err := cache.SaveTrendingPool(ctx, nil); err != nil {
		t.Fatalf("SaveTrendingPool empty error: %v", err)
	}
	ids, _ = cache.GetTrendingPool(ctx)
	if len(ids) != 0 {
		t.Fatalf("expected 0 ids after empty save, got %d", len(ids))
	}
}

func TestTrendingPoolCache_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Nil client
	nilCache := redis.NewTrendingPoolCache(nil)
	if err := nilCache.SaveTrendingPool(ctx, []string{"p1"}); err != nil {
		t.Errorf("unexpected error on nil client SaveTrendingPool: %v", err)
	}
	ids, err := nilCache.GetTrendingPool(ctx)
	if err != nil || len(ids) != 0 {
		t.Errorf("expected empty trending pool on nil client, got %v", ids)
	}

	// Closed client error handling
	client, cleanup := setupTestRedis(t)
	cache := redis.NewTrendingPoolCache(client)
	cleanup() // close client immediately

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = cache.GetTrendingPool(ctxTimeout)
	if err == nil {
		t.Errorf("expected error when redis client is closed on GetTrendingPool")
	}

	if err := cache.SaveTrendingPool(ctxTimeout, []string{"hot-1"}); err == nil {
		t.Errorf("expected error when redis client is closed on SaveTrendingPool")
	}
}
