package port

import (
	"context"
	"time"
)

// NotificationDeduplicator prevents duplicate notification spam (e.g., repeated like/unlike or follow/unfollow)
// within a configurable TTL window.
type NotificationDeduplicator interface {
	// Allow returns true if this dedup key has not been seen within ttl (and marks it as seen).
	Allow(ctx context.Context, key string, ttl time.Duration) (bool, error)
}
