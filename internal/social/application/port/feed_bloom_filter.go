package port

import (
	"context"
	"time"
)

// FeedBloomFilter defines operations for seen item deduplication via Bloom filter.
type FeedBloomFilter interface {
	FilterUnseen(ctx context.Context, userID string, itemIDs []string) ([]string, error)
	AddSeen(ctx context.Context, userID string, itemIDs []string, ttl time.Duration) error
}
