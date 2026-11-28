package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

var _ port.PushCooldownService = (*PushCooldownService)(nil)

// PushCooldownService provides Redis-backed distributed cooldown locking for push notifications.
type PushCooldownService struct {
	client *goredis.Client
}

// NewPushCooldownService creates a new PushCooldownService.
func NewPushCooldownService(client *goredis.Client) *PushCooldownService {
	return &PushCooldownService{client: client}
}

func (s *PushCooldownService) key(feedID, recipientID string) string {
	return fmt.Sprintf("push_cooldown:%s:%s", feedID, recipientID)
}

// CheckAndLock returns true if push is allowed (key was not present and has now been locked for ttl).
// If Redis is unavailable or unconfigured, it gracefully fails open (returns true) so critical pushes aren't blocked.
func (s *PushCooldownService) CheckAndLock(ctx context.Context, feedID, recipientID string, ttl time.Duration) (bool, error) {
	if s.client == nil {
		return true, nil
	}
	k := s.key(feedID, recipientID)
	ok, err := s.client.SetNX(ctx, k, "1", ttl).Result()
	if err != nil {
		return true, fmt.Errorf("redis check and lock %s: %w", k, err)
	}
	return ok, nil
}

// CheckAndLockBatch checks and acquires cooldown locks for multiple recipients in a single Redis round-trip via pipeline.
// If Redis is unavailable or fails, it gracefully fails open (returns all recipientIDs) so notifications are not dropped.
func (s *PushCooldownService) CheckAndLockBatch(
	ctx context.Context,
	feedID string,
	recipientIDs []string,
	ttl time.Duration,
) ([]string, error) {
	if s.client == nil || len(recipientIDs) == 0 {
		return recipientIDs, nil
	}

	pipe := s.client.Pipeline()
	cmds := make(map[string]*goredis.BoolCmd, len(recipientIDs))
	for _, id := range recipientIDs {
		cmds[id] = pipe.SetNX(ctx, s.key(feedID, id), "1", ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return recipientIDs, fmt.Errorf("redis check and lock batch feed %s: %w", feedID, err)
	}

	allowed := make([]string, 0, len(recipientIDs))
	for _, id := range recipientIDs {
		cmd := cmds[id]
		if cmd != nil && cmd.Val() {
			allowed = append(allowed, id)
		}
	}
	return allowed, nil
}

// ReleaseLock deletes the cooldown key immediately if push delivery fails, allowing subsequent events to retry.
func (s *PushCooldownService) ReleaseLock(ctx context.Context, feedID, recipientID string) error {
	if s.client == nil {
		return nil
	}
	k := s.key(feedID, recipientID)
	if err := s.client.Del(ctx, k).Err(); err != nil {
		return fmt.Errorf("redis release lock %s: %w", k, err)
	}
	return nil
}

// ReleaseLockBatch deletes cooldown keys for multiple recipients in a single Redis DEL command.
func (s *PushCooldownService) ReleaseLockBatch(ctx context.Context, feedID string, recipientIDs []string) error {
	if s.client == nil || len(recipientIDs) == 0 {
		return nil
	}
	keys := make([]string, len(recipientIDs))
	for i, id := range recipientIDs {
		keys[i] = s.key(feedID, id)
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis release lock batch feed %s: %w", feedID, err)
	}
	return nil
}
