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
	keyReactionFlushQueue  = "social:reaction:flush_queue"
	keyReactionPendingData = "social:reaction:pending:%s:%s"
	visibilityLockDuration = 30 * time.Second

	// scriptPopMaturedReaction atomically claims tasks whose score <= max_score
	// by advancing their score to lock_score (visibility timeout), preventing
	// concurrent BE instances from claiming the same tasks simultaneously.
	scriptPopMaturedReaction = `
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

// ReactionDebounceBuffer implements port.ReactionDebounceBuffer using Redis ZSET and Key-Value.
type ReactionDebounceBuffer struct {
	client    *redis.Client
	popScript *redis.Script
}

var _ port.ReactionDebounceBuffer = (*ReactionDebounceBuffer)(nil)

// NewReactionDebounceBuffer creates a new ReactionDebounceBuffer.
func NewReactionDebounceBuffer(client *redis.Client) *ReactionDebounceBuffer {
	return &ReactionDebounceBuffer{
		client:    client,
		popScript: redis.NewScript(scriptPopMaturedReaction),
	}
}

// BufferReaction stores the target reaction state and updates the flush deadline in the ZSET.
func (b *ReactionDebounceBuffer) BufferReaction(
	ctx context.Context,
	userID, feedItemID, reactionType string,
	holdDuration, ttl time.Duration,
) error {
	if b.client == nil {
		return errors.New("redis client is nil")
	}

	member := userID + ":" + feedItemID
	dataKey := fmt.Sprintf(keyReactionPendingData, userID, feedItemID)
	deadline := time.Now().Add(holdDuration).Unix()

	pipe := b.client.TxPipeline()
	pipe.Set(ctx, dataKey, reactionType, ttl)
	pipe.ZAdd(ctx, keyReactionFlushQueue, redis.Z{
		Score:  float64(deadline),
		Member: member,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("buffer reaction in redis: %w", err)
	}

	return nil
}

// GetPendingReaction retrieves the in-flight reaction from Redis if present.
func (b *ReactionDebounceBuffer) GetPendingReaction(
	ctx context.Context,
	userID, feedItemID string,
) (reactionType string, exists bool, err error) {
	if b.client == nil {
		return "", false, nil
	}

	dataKey := fmt.Sprintf(keyReactionPendingData, userID, feedItemID)
	val, err := b.client.Get(ctx, dataKey).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get pending reaction: %w", err)
	}

	return val, true, nil
}

// PopMaturedReactions atomically claims matured reaction tasks via Lua script,
// preventing concurrent worker instances from processing identical tasks.
func (b *ReactionDebounceBuffer) PopMaturedReactions(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]port.ReactionFlushTask, error) {
	if b.client == nil || b.popScript == nil {
		return nil, nil
	}

	maxScore := strconv.FormatInt(now.Unix(), 10)
	lockScore := strconv.FormatInt(now.Add(visibilityLockDuration).Unix(), 10)

	// Atomic claim via Lua: grabs matured members and shifts their score forward
	rawResult, err := b.popScript.Run(
		ctx,
		b.client,
		[]string{keyReactionFlushQueue},
		maxScore,
		lockScore,
		limit,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("atomic claim matured reaction tasks: %w", err)
	}

	rawSlice, ok := rawResult.([]interface{})
	if !ok || len(rawSlice) == 0 {
		return nil, nil
	}

	tasks := make([]port.ReactionFlushTask, 0, len(rawSlice))
	dataKeys := make([]string, 0, len(rawSlice))
	validMembers := make([][2]string, 0, len(rawSlice))

	for _, item := range rawSlice {
		m, isStr := item.(string)
		if !isStr {
			continue
		}
		parts := strings.SplitN(m, ":", 2)
		if len(parts) != 2 {
			_ = b.client.ZRem(ctx, keyReactionFlushQueue, m).Err()
			continue
		}
		dataKeys = append(dataKeys, fmt.Sprintf(keyReactionPendingData, parts[0], parts[1]))
		validMembers = append(validMembers, [2]string{parts[0], parts[1]})
	}

	if len(dataKeys) == 0 {
		return nil, nil
	}

	// 1 Single Network Roundtrip để lấy toàn bộ dữ liệu của tất cả task
	values, err := b.client.MGet(ctx, dataKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget pending reactions: %w", err)
	}

	var orphanedMembers []interface{}
	for i, val := range values {
		uID, fID := validMembers[i][0], validMembers[i][1]
		if val == nil {
			// Key dữ liệu đã hết hạn hoặc không tồn tại -> dọn khỏi ZSET
			orphanedMembers = append(orphanedMembers, uID+":"+fID)
			continue
		}

		reactionType, ok := val.(string)
		if !ok {
			continue
		}

		tasks = append(tasks, port.ReactionFlushTask{
			UserID:       uID,
			FeedItemID:   fID,
			ReactionType: reactionType,
		})
	}

	if len(orphanedMembers) > 0 {
		_ = b.client.ZRem(ctx, keyReactionFlushQueue, orphanedMembers...).Err()
	}

	return tasks, nil
}

// AckReaction removes the task from the ZSET queue and deletes its pending state.
func (b *ReactionDebounceBuffer) AckReaction(ctx context.Context, userID, feedItemID string) error {
	if b.client == nil {
		return nil
	}

	member := userID + ":" + feedItemID
	dataKey := fmt.Sprintf(keyReactionPendingData, userID, feedItemID)

	pipe := b.client.TxPipeline()
	pipe.ZRem(ctx, keyReactionFlushQueue, member)
	pipe.Del(ctx, dataKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("ack reaction debounce: %w", err)
	}

	return nil
}
