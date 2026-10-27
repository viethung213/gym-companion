package redis_test

import (
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	sharedRedis "github.com/viethung213/gym-companion/internal/shared/redis"
)

func TestSharedRedisRegistry(t *testing.T) {
	mr := miniredis.RunT(t)

	// Set REDIS_ADDR to miniredis address
	originalAddr := os.Getenv("REDIS_ADDR")
	defer os.Setenv("REDIS_ADDR", originalAddr)
	os.Setenv("REDIS_ADDR", mr.Addr())

	registry := sharedRedis.GetRegistry()
	defer registry.CloseAll()

	client, err := registry.GetClient()
	if err != nil {
		t.Fatalf("expected successful connection to miniredis, got: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil redis.Client")
	}

	// Singleton check
	client2, err := registry.GetClient()
	if err != nil {
		t.Fatalf("second call err: %v", err)
	}
	if client != client2 {
		t.Errorf("expected same singleton client instance")
	}

	// CloseAll check
	registry.CloseAll()
}
