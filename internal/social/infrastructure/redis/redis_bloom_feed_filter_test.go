package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/redis"
)

func setupTestRedis(t *testing.T) (*goredis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	bloomStore := make(map[string]map[string]bool)
	var mu sync.Mutex

	_ = mr.Server().Register("BF.MADD", func(c *server.Peer, cmd string, args []string) {
		if len(args) < 2 {
			c.WriteError("ERR wrong number of arguments")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		key := args[0]
		if bloomStore[key] == nil {
			bloomStore[key] = make(map[string]bool)
		}
		c.WriteLen(len(args) - 1)
		for _, item := range args[1:] {
			if bloomStore[key][item] {
				c.WriteInt(0)
			} else {
				bloomStore[key][item] = true
				c.WriteInt(1)
			}
		}
	})

	_ = mr.Server().Register("BF.MEXISTS", func(c *server.Peer, cmd string, args []string) {
		if len(args) < 2 {
			c.WriteError("ERR wrong number of arguments")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		key := args[0]
		c.WriteLen(len(args) - 1)
		for _, item := range args[1:] {
			if bloomStore[key] != nil && bloomStore[key][item] {
				c.WriteInt(1)
			} else {
				c.WriteInt(0)
			}
		}
	})

	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		_ = client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestRedisBloomFeedFilter_FilterUnseenAndAddSeen(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	filter := redis.NewBloomFeedFilter(client)
	ctx := context.Background()
	userID := "user-123"

	// Initially, all items are unseen
	items := []string{"post-1", "post-2", "post-3"}
	unseen, err := filter.FilterUnseen(ctx, userID, items)
	if err != nil {
		t.Fatalf("FilterUnseen unexpected error: %v", err)
	}
	if len(unseen) != 3 {
		t.Fatalf("expected 3 unseen items, got %d", len(unseen))
	}

	// Mark post-1 and post-2 as seen
	if err := filter.AddSeen(ctx, userID, []string{"post-1", "post-2"}, 24*time.Hour); err != nil {
		t.Fatalf("AddSeen unexpected error: %v", err)
	}

	// Now check again: post-1 and post-2 should be filtered out
	unseenAfter, err := filter.FilterUnseen(ctx, userID, items)
	if err != nil {
		t.Fatalf("FilterUnseen after AddSeen error: %v", err)
	}
	if len(unseenAfter) != 1 || unseenAfter[0] != "post-3" {
		t.Fatalf("expected only post-3 to be unseen, got %v", unseenAfter)
	}
}

func TestRedisBloomFeedFilter_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Nil client
	nilFilter := redis.NewBloomFeedFilter(nil)
	items, err := nilFilter.FilterUnseen(ctx, "user", []string{"p1"})
	if err != nil || len(items) != 1 {
		t.Errorf("expected 1 item on nil client, got %v, err %v", items, err)
	}
	if err := nilFilter.AddSeen(ctx, "user", []string{"p1"}, time.Hour); err != nil {
		t.Errorf("unexpected error on nil client AddSeen: %v", err)
	}

	// Empty input arguments with valid client
	client, cleanup := setupTestRedis(t)
	defer cleanup()
	validFilter := redis.NewBloomFeedFilter(client)

	emptyUnseen, _ := validFilter.FilterUnseen(ctx, "", []string{"p1"})
	if len(emptyUnseen) != 1 {
		t.Errorf("expected 1 item when userID is empty")
	}

	emptyItems, _ := validFilter.FilterUnseen(ctx, "user", nil)
	if len(emptyItems) != 0 {
		t.Errorf("expected 0 items when itemIDs is empty")
	}

	if err := validFilter.AddSeen(ctx, "", []string{"p1"}, time.Hour); err != nil {
		t.Errorf("unexpected error on empty userID AddSeen: %v", err)
	}
	if err := validFilter.AddSeen(ctx, "user", nil, time.Hour); err != nil {
		t.Errorf("unexpected error on empty itemIDs AddSeen: %v", err)
	}
}

func TestRedisBloomFeedFilter_FailOpenOnError(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	filter := redis.NewBloomFeedFilter(client)
	ctx := context.Background()

	// Close redis to simulate network failure or redis outage
	cleanup()

	items := []string{"post-1", "post-2"}
	unseen, err := filter.FilterUnseen(ctx, "user-1", items)
	if err != nil {
		t.Fatalf("expected nil error on fail-open, got %v", err)
	}
	if len(unseen) != 2 {
		t.Fatalf("expected fail-open to return all 2 items, got %d", len(unseen))
	}
}
