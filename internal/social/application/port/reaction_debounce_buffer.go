package port

import (
	"context"
	"time"
)

// ReactionFlushTask represents a debounced reaction task ready to be persisted.
type ReactionFlushTask struct {
	UserID       string
	FeedItemID   string
	ReactionType string // Target reaction (e.g., "LIKE", "FIRE") or empty string if toggled off
}

// ReactionDebounceBuffer defines the contract for buffering and debouncing rapid reaction changes.
type ReactionDebounceBuffer interface {
	// BufferReaction registers or updates the desired reaction for a user on a feed item,
	// setting a scheduled flush deadline (now + holdDuration) and updating key TTL.
	BufferReaction(ctx context.Context, userID, feedItemID, reactionType string, holdDuration, ttl time.Duration) error

	// GetPendingReaction retrieves the current in-flight reaction if present in the buffer.
	GetPendingReaction(ctx context.Context, userID, feedItemID string) (reactionType string, exists bool, err error)

	// PopMaturedReactions retrieves tasks whose flush deadline is at or before the given timestamp.
	PopMaturedReactions(ctx context.Context, now time.Time, limit int) ([]ReactionFlushTask, error)

	// AckReaction removes the task from the scheduled queue and cleans up the pending data.
	AckReaction(ctx context.Context, userID, feedItemID string) error
}
