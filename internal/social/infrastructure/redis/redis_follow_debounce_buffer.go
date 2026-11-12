package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

const (
	keyFollowFlushQueue          = "social:follow:flush_queue"
	keyFollowPendingData         = "social:follow:pending:%s:%s"
	followVisibilityLockDuration = 30 * time.Second

	// scriptPopMaturedFollow atomically claims tasks whose score <= max_score
	// by advancing their score to lock_score (visibility timeout), preventing
	// concurrent BE instances from claiming the same tasks simultaneously.
	scriptPopMaturedFollow = `
local queue = KEYS[1]
local max_score = ARGV[1]
local lock_score = ARGV[2]
local limit = tonumber(ARGV[3])

local members = redis.call('ZRANGEBYSCORE', queue, '-inf', max_score, 'LIMIT', 0, limit)
if #members == 0 then
    return {}
end

for _, member in ipairs(members) do
    redis.call('ZADD', queue, lock_score, member)
end

return members
`
)

// FollowDebounceBuffer implements port.FollowDebounceBuffer using Redis ZSET and Key-Value.
type FollowDebounceBuffer struct {
	client    *redis.Client
	popScript *redis.Script
}

var _ port.FollowDebounceBuffer = (*FollowDebounceBuffer)(nil)

// NewFollowDebounceBuffer creates a new FollowDebounceBuffer.
func NewFollowDebounceBuffer(client *redis.Client) *FollowDebounceBuffer {
	return &FollowDebounceBuffer{
		client:    client,
		popScript: redis.NewScript(scriptPopMaturedFollow),
	}
}

// BufferFollow stores the target follow state and updates the flush deadline in the ZSET.
func (b *FollowDebounceBuffer) BufferFollow(
	ctx context.Context,
	followerID, followingID string,
	isFollowing bool,
	holdDuration, ttl time.Duration,
) error {
	if b.client == nil {
		return errors.New("redis client is nil")
	}

	member := followerID + ":" + followingID
	dataKey := fmt.Sprintf(keyFollowPendingData, followerID, followingID)
	deadline := time.Now().Add(holdDuration).Unix()

	val := "0"
	if isFollowing {
		val = "1"
	}

	pipe := b.client.TxPipeline()
	pipe.Set(ctx, dataKey, val, ttl)
	pipe.ZAdd(ctx, keyFollowFlushQueue, redis.Z{
		Score:  float64(deadline),
		Member: member,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("buffer follow in redis: %w", err)
	}

	return nil
}

// GetPendingFollow retrieves the in-flight follow status from Redis if present.
func (b *FollowDebounceBuffer) GetPendingFollow(
	ctx context.Context,
	followerID, followingID string,
) (isFollowing, exists bool, err error) {
	if b.client == nil {
		return false, false, nil
	}

	dataKey := fmt.Sprintf(keyFollowPendingData, followerID, followingID)
	val, err := b.client.Get(ctx, dataKey).Result()
	if errors.Is(err, redis.Nil) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("get pending follow: %w", err)
	}

	return val == "1", true, nil
}

// PopMaturedFollows atomically claims matured follow tasks via Lua script,
// preventing concurrent worker instances from processing identical tasks.
func (b *FollowDebounceBuffer) PopMaturedFollows(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]port.FollowFlushTask, error) {
	if b.client == nil || b.popScript == nil {
		return nil, nil
	}

	maxScore := strconv.FormatInt(now.Unix(), 10)
	lockScore := strconv.FormatInt(now.Add(followVisibilityLockDuration).Unix(), 10)

	// Atomic claim via Lua: grabs matured members and shifts their score forward
	rawResult, err := b.popScript.Run(
		ctx,
		b.client,
		[]string{keyFollowFlushQueue},
		maxScore,
		lockScore,
		limit,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("atomic claim matured follow tasks: %w", err)
	}

	rawSlice, ok := rawResult.([]interface{})
	if !ok || len(rawSlice) == 0 {
		return nil, nil
	}

	tasks := make([]port.FollowFlushTask, 0, len(rawSlice))
	dataKeys := make([]string, 0, len(rawSlice))
	validMembers := make([][2]string, 0, len(rawSlice))

	for _, item := range rawSlice {
		m, isStr := item.(string)
		if !isStr {
			continue
		}
		parts := strings.SplitN(m, ":", 2)
		if len(parts) != 2 {
			_ = b.client.ZRem(ctx, keyFollowFlushQueue, m).Err()
			continue
		}
		dataKeys = append(dataKeys, fmt.Sprintf(keyFollowPendingData, parts[0], parts[1]))
		validMembers = append(validMembers, [2]string{parts[0], parts[1]})
	}

	if len(dataKeys) == 0 {
		return nil, nil
	}

	// 1 Single Network Roundtrip để lấy toàn bộ trạng thái Follow
	values, err := b.client.MGet(ctx, dataKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget pending follows: %w", err)
	}

	var orphanedMembers []interface{}
	for i, val := range values {
		fID, toID := validMembers[i][0], validMembers[i][1]
		if val == nil {
			orphanedMembers = append(orphanedMembers, fID+":"+toID)
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			continue
		}

		tasks = append(tasks, port.FollowFlushTask{
			FollowerID:  fID,
			FollowingID: toID,
			IsFollowing: strVal == "1",
		})
	}

	if len(orphanedMembers) > 0 {
		_ = b.client.ZRem(ctx, keyFollowFlushQueue, orphanedMembers...).Err()
	}

	return tasks, nil
}

// AckFollow removes the task from the ZSET queue and deletes its pending state.
func (b *FollowDebounceBuffer) AckFollow(ctx context.Context, followerID, followingID string) error {
	if b.client == nil {
		return nil
	}

	member := followerID + ":" + followingID
	dataKey := fmt.Sprintf(keyFollowPendingData, followerID, followingID)

	pipe := b.client.TxPipeline()
	pipe.ZRem(ctx, keyFollowFlushQueue, member)
	pipe.Del(ctx, dataKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("ack follow debounce: %w", err)
	}

	return nil
}
