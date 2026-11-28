package redis

import (
	"context"
	"fmt"
	"math"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
)

const (
	// releaseLockLua deletes the lock only when it is still owned by the caller.
	releaseLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
	// extendLockLua renews the lock TTL (ms) only when it is still owned by the caller.
	extendLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`
	// markReadyLua publishes the rebuild only if this node still owns the lock and the ZSET still
	// carries its build marker, i.e. the key was not deleted or evicted while rebuilding.
	markReadyLua = `
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
	return 0
end
if not redis.call("ZSCORE", KEYS[2], ARGV[2]) then
	return 0
end
redis.call("ZREM", KEYS[2], ARGV[2])
redis.call("ZADD", KEYS[2], "+inf", ARGV[3])
return 1
`
)

// LeaderboardProjection writes the XP leaderboard read model into a Redis Sorted Set.
type LeaderboardProjection struct {
	client      *goredis.Client
	nodeID      string
	releaseLock *goredis.Script
	extendLock  *goredis.Script
	markReady   *goredis.Script
}

// Compile-time interface check.
var _ port.LeaderboardProjection = (*LeaderboardProjection)(nil)

// NewLeaderboardProjection constructs a LeaderboardProjection; nodeID identifies this process as
// the rebuild lock owner.
func NewLeaderboardProjection(client *goredis.Client, nodeID string) *LeaderboardProjection {
	return &LeaderboardProjection{
		client:      client,
		nodeID:      nodeID,
		releaseLock: goredis.NewScript(releaseLockLua),
		extendLock:  goredis.NewScript(extendLockLua),
		markReady:   goredis.NewScript(markReadyLua),
	}
}

// UpsertScores applies scores with ZADD LT: a member is added when absent and only ever moves up,
// which makes replays and out-of-order deliveries harmless.
func (p *LeaderboardProjection) UpsertScores(
	ctx context.Context,
	scores []entity.LeaderboardScore,
) error {
	members := make([]goredis.Z, 0, len(scores))
	for _, s := range scores {
		member := normalizeMember(s.UserID)
		if member == "" || isInternalMember(member) {
			continue
		}
		members = append(members, goredis.Z{Score: encodeScore(s.Xp, s.UpdatedAt), Member: member})
	}
	if len(members) == 0 {
		return nil
	}

	if err := p.client.ZAddLT(ctx, leaderboardKey, members...).Err(); err != nil {
		return redisError("zadd lt leaderboard scores", err)
	}
	return nil
}

// Reset deletes the whole read model, including the ready sentinel.
func (p *LeaderboardProjection) Reset(ctx context.Context) error {
	if err := p.client.Del(ctx, leaderboardKey).Err(); err != nil {
		return redisError("reset leaderboard", err)
	}
	return nil
}

// BeginRebuild stamps the ZSET with this node's build marker; MarkReady requires it to survive.
func (p *LeaderboardProjection) BeginRebuild(ctx context.Context) error {
	marker := goredis.Z{Score: math.Inf(1), Member: p.buildMarker()}
	if err := p.client.ZAdd(ctx, leaderboardKey, marker).Err(); err != nil {
		return redisError("begin leaderboard rebuild", err)
	}
	return nil
}

// MarkReady atomically swaps this node's build marker for the ready sentinel. It fails with
// ErrRebuildInterrupted when the lock or the marker was lost, so a partial set is never published.
func (p *LeaderboardProjection) MarkReady(ctx context.Context) error {
	keys := []string{rebuildLockKey, leaderboardKey}
	published, err := p.markReady.Run(ctx, p.client, keys, p.nodeID, p.buildMarker(), ReadySentinel).
		Int()
	if err != nil {
		return redisError("mark leaderboard ready", err)
	}
	if published == 0 {
		return fmt.Errorf("mark leaderboard ready: %w", ErrRebuildInterrupted)
	}
	return nil
}

// TryAcquireRebuildLock takes the rebuild lock for this node if no other node holds it.
func (p *LeaderboardProjection) TryAcquireRebuildLock(ctx context.Context) (bool, error) {
	ok, err := p.client.SetNX(ctx, rebuildLockKey, p.nodeID, rebuildLockTTL).Result()
	if err != nil {
		return false, redisError("acquire leaderboard rebuild lock", err)
	}
	return ok, nil
}

// ExtendRebuildLock renews the rebuild lock; ErrRebuildInterrupted when another node owns it.
func (p *LeaderboardProjection) ExtendRebuildLock(ctx context.Context) error {
	ttlMillis := rebuildLockTTL.Milliseconds()
	extended, err := p.extendLock.Run(ctx, p.client, []string{rebuildLockKey}, p.nodeID, ttlMillis).
		Int()
	if err != nil {
		return redisError("extend leaderboard rebuild lock", err)
	}
	if extended == 0 {
		return fmt.Errorf("extend leaderboard rebuild lock: %w", ErrRebuildInterrupted)
	}
	return nil
}

// ReleaseRebuildLock releases the rebuild lock if this node still owns it.
func (p *LeaderboardProjection) ReleaseRebuildLock(ctx context.Context) error {
	if err := p.releaseLock.Run(ctx, p.client, []string{rebuildLockKey}, p.nodeID).Err(); err != nil {
		return redisError("release leaderboard rebuild lock", err)
	}
	return nil
}

func (p *LeaderboardProjection) buildMarker() string {
	return buildMarkerPrefix + p.nodeID
}

// redisError tags a failure of a Redis command so callers can tell it from other failures.
func redisError(operation string, err error) error {
	return fmt.Errorf("%s: %w: %w", operation, ErrRedisUnavailable, err)
}
