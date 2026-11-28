package port

import (
	"context"
	"time"
)

// PushCooldownService manages cooldown locks to prevent notification spam on social entities (e.g. feed comments/reactions).
type PushCooldownService interface {
	// CheckAndLock returns true if push delivery is permitted (cooldown key did not exist and is now locked for ttl).
	// If cooldown is currently active, it returns false without acquiring a lock.
	CheckAndLock(ctx context.Context, feedID, recipientID string, ttl time.Duration) (bool, error)

	// CheckAndLockBatch checks and acquires cooldown locks for multiple recipients in a single Redis round-trip (e.g. pipeline).
	// It returns the slice of recipient IDs that were successfully locked (cooldown was not active).
	CheckAndLockBatch(ctx context.Context, feedID string, recipientIDs []string, ttl time.Duration) ([]string, error)

	// ReleaseLock immediately deletes the cooldown lock for (feedID, recipientID) if push delivery failed (self-healing).
	ReleaseLock(ctx context.Context, feedID, recipientID string) error

	// ReleaseLockBatch deletes cooldown locks for multiple recipients in a single Redis round-trip if group/broadcast delivery failed.
	ReleaseLockBatch(ctx context.Context, feedID string, recipientIDs []string) error
}
