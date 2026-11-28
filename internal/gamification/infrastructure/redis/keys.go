package redis

import (
	"errors"
	"strings"
	"time"
)

const (
	// leaderboardKey holds one member per user (score: see score_codec.go) plus internal markers.
	leaderboardKey = "gamification:leaderboard:xp"
	// rebuildLockKey guards against concurrent rebuilds across instances.
	rebuildLockKey = "gamification:leaderboard:xp:rebuild_lock"
	rebuildLockTTL = 120 * time.Second

	// ReadySentinel is a member stored inside the leaderboard ZSET with score +inf once a rebuild
	// completed. Keeping the marker in the same key means eviction or deletion removes both
	// together, so a partially re-created ZSET can never be served as complete.
	ReadySentinel = "__ready__"

	// internalMemberPrefix marks non-user members (ready sentinel, rebuild markers). User IDs are
	// UUIDs and never start with it.
	internalMemberPrefix = "__"
	buildMarkerPrefix    = "__building__:"
)

var (
	// ErrLeaderboardNotReady means the read model is missing or incomplete and must not be served.
	ErrLeaderboardNotReady = errors.New("redis leaderboard not ready")
	// ErrRedisUnavailable wraps failures talking to Redis itself (connection, timeout, protocol).
	ErrRedisUnavailable = errors.New("redis unavailable")
	// ErrRebuildInterrupted means a rebuild lost its lock or its ZSET while running.
	ErrRebuildInterrupted = errors.New("leaderboard rebuild interrupted")
)

func isInternalMember(member string) bool {
	return strings.HasPrefix(member, internalMemberPrefix)
}

// normalizeMember maps a user ID to its ZSET member; PostgreSQL renders UUIDs in lowercase.
func normalizeMember(userID string) string {
	return strings.ToLower(userID)
}
