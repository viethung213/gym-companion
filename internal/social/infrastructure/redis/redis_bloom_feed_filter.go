package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

const (
	keyPrefixBloomSeen = "social:seen:bf:"
)

// BloomFeedFilter implements port.FeedBloomFilter using native RedisBloom commands.
// It applies a fail-open strategy: if Redis is unreachable or returns an error,
// it gracefully returns the original items so the user's feed is never interrupted.
type BloomFeedFilter struct {
	client *redis.Client
}

var _ port.FeedBloomFilter = (*BloomFeedFilter)(nil)

// NewBloomFeedFilter creates a new BloomFeedFilter adapter.
func NewBloomFeedFilter(client *redis.Client) *BloomFeedFilter {
	return &BloomFeedFilter{client: client}
}

// FilterUnseen returns only the items that have not been seen by the given user.
// Fails open by returning all itemIDs if Redis is unavailable.
func (f *BloomFeedFilter) FilterUnseen(
	ctx context.Context,
	userID string,
	itemIDs []string,
) ([]string, error) {
	if len(itemIDs) == 0 || userID == "" || f.client == nil {
		return itemIDs, nil
	}

	key := keyPrefixBloomSeen + userID
	interfaces := make([]interface{}, len(itemIDs))
	for i, id := range itemIDs {
		interfaces[i] = id
	}

	cmd := f.client.BFMExists(ctx, key, interfaces...)
	seenFlags, err := cmd.Result()
	if err != nil {
		// Fail-open: return all items on Redis failure so feed continues to serve
		return itemIDs, nil
	}

	unseen := make([]string, 0, len(itemIDs))
	for i, isSeen := range seenFlags {
		if !isSeen {
			unseen = append(unseen, itemIDs[i])
		}
	}

	return unseen, nil
}

// AddSeen marks items as seen for the given user and sets expiration.
func (f *BloomFeedFilter) AddSeen(
	ctx context.Context,
	userID string,
	itemIDs []string,
	ttl time.Duration,
) error {
	if len(itemIDs) == 0 || userID == "" || f.client == nil {
		return nil
	}

	key := keyPrefixBloomSeen + userID
	interfaces := make([]interface{}, len(itemIDs))
	for i, id := range itemIDs {
		interfaces[i] = id
	}

	_, err := f.client.BFMAdd(ctx, key, interfaces...).Result()
	if err != nil {
		return fmt.Errorf("add items to bloom filter: %w", err)
	}

	if ttl > 0 {
		_ = f.client.Expire(ctx, key, ttl).Err()
	}

	return nil
}
