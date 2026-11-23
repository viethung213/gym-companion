package redis

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

type NotificationDeduplicator struct {
	client goredis.UniversalClient
}

var _ port.NotificationDeduplicator = (*NotificationDeduplicator)(nil)

func NewNotificationDeduplicator(client goredis.UniversalClient) *NotificationDeduplicator {
	return &NotificationDeduplicator{client: client}
}

// Allow uses Redis SETNX with TTL to suppress duplicate notifications within the window.
// Fails open on Redis error so transient cache issues never silently drop notifications.
func (d *NotificationDeduplicator) Allow(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if d == nil || d.client == nil || key == "" {
		return true, nil
	}
	ok, err := d.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		slog.WarnContext(ctx, "Redis notification dedup failed, failing open", slog.String("key", key), slog.Any("error", err))
		return true, err
	}
	return ok, nil
}
