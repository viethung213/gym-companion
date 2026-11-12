package port

import (
	"context"
	"time"
)

// FollowFlushTask represents a debounced follow/unfollow task ready to be persisted.
type FollowFlushTask struct {
	FollowerID  string
	FollowingID string
	IsFollowing bool // true = Follow, false = Unfollow
}

// FollowDebounceBuffer defines the contract for buffering and debouncing rapid follow/unfollow changes.
type FollowDebounceBuffer interface {
	// BufferFollow registers or updates the desired follow state for a follower/following pair,
	// setting a scheduled flush deadline (now + holdDuration) and updating key TTL.
	BufferFollow(ctx context.Context, followerID, followingID string, isFollowing bool, holdDuration, ttl time.Duration) error

	// GetPendingFollow retrieves the current in-flight follow state if present in the buffer.
	GetPendingFollow(ctx context.Context, followerID, followingID string) (isFollowing, exists bool, err error)

	// PopMaturedFollows retrieves tasks whose flush deadline is at or before the given timestamp.
	PopMaturedFollows(ctx context.Context, now time.Time, limit int) ([]FollowFlushTask, error)

	// AckFollow removes the task from the scheduled queue and cleans up the pending data.
	AckFollow(ctx context.Context, followerID, followingID string) error
}
