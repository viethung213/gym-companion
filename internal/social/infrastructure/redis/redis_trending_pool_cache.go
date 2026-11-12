package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

const (
	keyTrendingPool     = "global:trending_pool"
	keyTrendingPoolTemp = "global:trending_pool_temp"
)

// TrendingPoolCache implements port.TrendingPoolCache using Redis List operations.
type TrendingPoolCache struct {
	client *redis.Client
}

var _ port.TrendingPoolCache = (*TrendingPoolCache)(nil)

// NewTrendingPoolCache creates a new TrendingPoolCache adapter.
func NewTrendingPoolCache(client *redis.Client) *TrendingPoolCache {
	return &TrendingPoolCache{client: client}
}

// SaveTrendingPool atomically replaces the global trending pool list with new items.
func (c *TrendingPoolCache) SaveTrendingPool(ctx context.Context, itemIDs []string) error {
	if c.client == nil {
		return nil
	}

	_ = c.client.Del(ctx, keyTrendingPoolTemp).Err()
	if len(itemIDs) > 0 {
		interfaces := make([]interface{}, len(itemIDs))
		for i, id := range itemIDs {
			interfaces[i] = id
		}
		if err := c.client.RPush(ctx, keyTrendingPoolTemp, interfaces...).Err(); err != nil {
			return fmt.Errorf("rpush trending pool temp: %w", err)
		}
		if err := c.client.Rename(ctx, keyTrendingPoolTemp, keyTrendingPool).Err(); err != nil {
			return fmt.Errorf("rename trending pool: %w", err)
		}
	} else {
		_ = c.client.Del(ctx, keyTrendingPool).Err()
	}

	return nil
}

// GetTrendingPool retrieves the current cached trending item IDs.
func (c *TrendingPoolCache) GetTrendingPool(ctx context.Context) ([]string, error) {
	if c.client == nil {
		return nil, nil
	}

	ids, err := c.client.LRange(ctx, keyTrendingPool, 0, -1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("lrange trending pool: %w", err)
	}

	return ids, nil
}
