package redis

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Registry manages the shared Redis client connection pool for all modules.
type Registry struct {
	mu     sync.RWMutex
	client *redis.Client
}

var (
	//nolint:gochecknoglobals // Singleton Redis registry instance.
	instance *Registry
	//nolint:gochecknoglobals // Ensures singleton initialization once.
	once sync.Once
)

// GetRegistry returns the singleton instance of the Redis connection Registry.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{}
	})
	return instance
}

// GetClient retrieves or initializes the shared Redis client connection pool.
// It checks REDIS_ADDR or REDIS_URL environment variables.
func (r *Registry) GetClient() (*redis.Client, error) {
	r.mu.RLock()
	if r.client != nil {
		defer r.mu.RUnlock()
		return r.client, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		return r.client, nil
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_URL")
	}
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	var opts *redis.Options
	if strings.HasPrefix(redisAddr, "redis://") || strings.HasPrefix(redisAddr, "rediss://") {
		parsedOpts, parseErr := redis.ParseURL(redisAddr)
		if parseErr != nil {
			slog.Warn("Failed to parse REDIS_URL, falling back to Addr", "url", redisAddr, "error", parseErr)
			opts = &redis.Options{Addr: redisAddr}
		} else {
			opts = parsedOpts
		}
	} else {
		opts = &redis.Options{
			Addr: redisAddr,
		}
	}

	client := redis.NewClient(opts)

	// Verify connection with short timeout ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis at %s: %w", redisAddr, err)
	}

	r.client = client
	slog.Info("Connected to Redis successfully", "addr", redisAddr)
	return r.client, nil
}

// CloseAll cleanly terminates the active Redis connection pool.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		_ = r.client.Close()
		r.client = nil
		slog.Info("Closed shared Redis client pool")
	}
}
