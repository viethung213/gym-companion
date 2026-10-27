package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

const (
	defaultRoomTTL = 2 * time.Hour

	// registerScript atomically checks room membership, exercise compatibility, and capacity.
	// Returns 0 on success, 1 if room is full, 2 if exercise mismatches.
	registerScript = `
local existingEx = redis.call('GET', KEYS[3])
if existingEx and existingEx ~= false and existingEx ~= '' and ARGV[4] ~= '' and existingEx ~= ARGV[4] then
    return 2
end
local isMember = redis.call('SISMEMBER', KEYS[1], ARGV[1])
if isMember == 1 then
    return 0
end
local count = redis.call('SCARD', KEYS[1])
if count >= 2 then
    return 1
end
redis.call('SADD', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], ARGV[2])
if (not existingEx or existingEx == false or existingEx == '') and ARGV[4] ~= '' then
    redis.call('SET', KEYS[3], ARGV[4], 'EX', ARGV[2])
else
    redis.call('EXPIRE', KEYS[3], ARGV[2])
end
if ARGV[3] ~= '' then
    redis.call('HSET', KEYS[2], ARGV[1], ARGV[3])
    redis.call('EXPIRE', KEYS[2], ARGV[2])
end
return 0
`
)

// RedisLiveSyncHub implements port.SoloMatchLiveSyncPort using Redis.
// It ensures distributed multi-pod synchronization and atomic room admission.
type RedisLiveSyncHub struct {
	client *redis.Client
	ttl    time.Duration
}

var _ port.SoloMatchLiveSyncPort = (*RedisLiveSyncHub)(nil)

// NewRedisLiveSyncHub constructs a new RedisLiveSyncHub.
func NewRedisLiveSyncHub(client *redis.Client, ttl time.Duration) *RedisLiveSyncHub {
	if ttl <= 0 {
		ttl = defaultRoomTTL
	}
	return &RedisLiveSyncHub{
		client: client,
		ttl:    ttl,
	}
}

// RegisterParticipant atomically registers a user into the match room in Redis.
// Returns apperror.ErrMatchRoomFull if 2 distinct participants have already joined.
// Returns apperror.ErrMatchExerciseMismatch if the match room has a different exercise configured.
func (h *RedisLiveSyncHub) RegisterParticipant(ctx context.Context, matchID, userID, exerciseID string) error {
	partKey := "solo_match:" + matchID + ":participants"
	progKey := "solo_match:" + matchID + ":progress"
	metaKey := "solo_match:" + matchID + ":exercise"

	initialProgress := port.ParticipantProgress{
		UserID: userID,
		Status: "WAITING",
	}
	initJSON, err := json.Marshal(initialProgress)
	if err != nil {
		return fmt.Errorf("marshal initial progress: %w", err)
	}

	ttlSeconds := strconv.Itoa(int(h.ttl.Seconds()))
	res, err := h.client.Eval(ctx, registerScript, []string{partKey, progKey, metaKey}, userID, ttlSeconds, string(initJSON), exerciseID).Result()
	if err != nil {
		return fmt.Errorf("redis eval register participant: %w", err)
	}

	code, ok := res.(int64)
	if !ok {
		return fmt.Errorf("unexpected redis script return type: %T", res)
	}

	if code == 1 {
		return apperror.ErrMatchRoomFull
	}
	if code == 2 {
		return apperror.ErrMatchExerciseMismatch
	}

	// If room became ready (both participants joined), publish registration to stream to wake up waiting participants
	if ready, checkErr := h.IsMatchReady(ctx, matchID); checkErr == nil && ready {
		streamKey := "solo_match:" + matchID + ":stream"
		_ = h.client.Publish(ctx, streamKey, initJSON).Err()
	}

	return nil
}

// UnregisterParticipant updates a participant's status upon stream disconnection.
// If the participant has not finished the workout, their status becomes DISCONNECTED while preserving their progress,
// allowing them to reconnect as long as the match time ceiling has not expired.
// Only when BOTH participants have reached a terminal state (COMPLETED or ABORTED) is the Redis room purged.
func (h *RedisLiveSyncHub) UnregisterParticipant(ctx context.Context, matchID, userID string) error {
	partKey := "solo_match:" + matchID + ":participants"
	progKey := "solo_match:" + matchID + ":progress"
	metaKey := "solo_match:" + matchID + ":exercise"

	payload, err := h.client.HGet(ctx, progKey, userID).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis hget progress on unregister: %w", err)
	}

	var prog port.ParticipantProgress
	if err == nil && payload != "" {
		if unmarshalErr := json.Unmarshal([]byte(payload), &prog); unmarshalErr != nil {
			return fmt.Errorf("unmarshal participant progress on unregister: %w", unmarshalErr)
		}
	} else {
		prog = port.ParticipantProgress{
			UserID: userID,
			Status: "DISCONNECTED",
		}
	}

	wasFinished := prog.Status == "COMPLETED" || prog.Status == "ABORTED"
	if !wasFinished {
		// Chưa hoàn thành -> Đánh dấu DISCONNECTED và thời điểm mất kết nối để tính Grace Period
		prog.Status = "DISCONNECTED"
		prog.DisconnectedAt = time.Now().Unix()
	}

	updatedPayload, err := json.Marshal(prog)
	if err != nil {
		return fmt.Errorf("marshal participant progress on unregister: %w", err)
	}

	streamKey := "solo_match:" + matchID + ":stream"
	pipe := h.client.Pipeline()
	pipe.HSet(ctx, progKey, userID, updatedPayload)
	pipe.Expire(ctx, progKey, h.ttl)
	pipe.Expire(ctx, partKey, h.ttl)
	pipe.Expire(ctx, metaKey, h.ttl)
	pipe.Publish(ctx, streamKey, updatedPayload)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline unregister participant: %w", err)
	}

	members, err := h.client.SMembers(ctx, partKey).Result()
	if err != nil {
		return fmt.Errorf("redis smembers on unregister check: %w", err)
	}

	// Nếu trận đấu chưa đủ 2 người (đang trong giai đoạn check-in 60s) hoặc bản thân người này CHƯA hoàn thành bài tập:
	// Bắt buộc GIỮ PHÒNG để cho phép người dùng reconnect hoặc đợi người thứ 2 vào.
	// Quyền hủy phòng nếu quá 60s không đủ người thuộc về ready_deadline của solo_match (phát SoloMatchCancelled -> DelRoom).
	if len(members) < 2 || !wasFinished {
		return nil
	}

	// Nếu trận đấu đã đủ 2 người và người này ĐÃ hoàn thành -> Kiểm tra xem đối thủ cũng đã hoàn thành chưa

	allFinished := true
	for _, mUID := range members {
		var memberProg port.ParticipantProgress
		var mPayload string
		if mUID == userID {
			mPayload = string(updatedPayload)
		} else {
			mPayload, err = h.client.HGet(ctx, progKey, mUID).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return fmt.Errorf("redis hget member progress on unregister check: %w", err)
			}
		}

		if mPayload == "" {
			allFinished = false
			break
		}

		if err := json.Unmarshal([]byte(mPayload), &memberProg); err != nil {
			allFinished = false
			break
		}

		// Chỉ khi đối thủ đã COMPLETED hoặc ABORTED thì mới coi là xong
		if memberProg.Status != "COMPLETED" && memberProg.Status != "ABORTED" {
			allFinished = false
			break
		}
	}

	// Chỉ xóa phòng khi CẢ HAI ĐỀU ĐÃ HOÀN THÀNH / ABORTED
	if allFinished {
		if err := h.client.Del(ctx, partKey, progKey, metaKey).Err(); err != nil {
			return fmt.Errorf("redis del match room on completion: %w", err)
		}
	}

	return nil
}

// IsMatchReady checks whether at least 2 participants have joined the match room.
func (h *RedisLiveSyncHub) IsMatchReady(ctx context.Context, matchID string) (bool, error) {
	partKey := "solo_match:" + matchID + ":participants"
	count, err := h.client.SCard(ctx, partKey).Result()
	if err != nil {
		return false, fmt.Errorf("redis scard participants: %w", err)
	}
	return count >= 2, nil
}

// ParticipantCount returns the number of participants currently registered in the room.
func (h *RedisLiveSyncHub) ParticipantCount(ctx context.Context, matchID string) (int, error) {
	partKey := "solo_match:" + matchID + ":participants"
	count, err := h.client.SCard(ctx, partKey).Result()
	if err != nil {
		return 0, fmt.Errorf("redis scard participants: %w", err)
	}
	return int(count), nil
}

// PublishMyProgress stores current score/progress in Redis and broadcasts to opponents via Pub/Sub.
// Important: Only evaluated score/progress is stored; raw skeleton coordinates are never stored in Redis.
func (h *RedisLiveSyncHub) PublishMyProgress(ctx context.Context, matchID string, progress port.ParticipantProgress) error {
	payload, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal participant progress: %w", err)
	}

	progKey := "solo_match:" + matchID + ":progress"
	streamKey := "solo_match:" + matchID + ":stream"

	pipe := h.client.Pipeline()
	pipe.HSet(ctx, progKey, progress.UserID, payload)
	pipe.Expire(ctx, progKey, h.ttl)
	pipe.Publish(ctx, streamKey, payload)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline publish progress: %w", err)
	}
	return nil
}

// GetMyProgress fetches the recorded progress of the specified user in the match room.
func (h *RedisLiveSyncHub) GetMyProgress(ctx context.Context, matchID, userID string) (*port.ParticipantProgress, error) {
	progKey := "solo_match:" + matchID + ":progress"
	payload, err := h.client.HGet(ctx, progKey, userID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis hget progress: %w", err)
	}
	if payload == "" {
		return nil, nil
	}
	var prog port.ParticipantProgress
	if err := json.Unmarshal([]byte(payload), &prog); err != nil {
		return nil, fmt.Errorf("unmarshal my progress: %w", err)
	}
	return &prog, nil
}

// GetOpponentProgress fetches the latest recorded progress of the single opponent in the 1v1 match.
func (h *RedisLiveSyncHub) GetOpponentProgress(ctx context.Context, matchID, myUserID string) (*port.ParticipantProgress, error) {
	progKey := "solo_match:" + matchID + ":progress"
	dataMap, err := h.client.HGetAll(ctx, progKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall progress: %w", err)
	}

	for uid, payload := range dataMap {
		if uid != myUserID {
			var prog port.ParticipantProgress
			if err := json.Unmarshal([]byte(payload), &prog); err != nil {
				return nil, fmt.Errorf("unmarshal opponent progress: %w", err)
			}
			return &prog, nil
		}
	}
	return nil, nil
}

// SubscribeMatchUpdates subscribes to updates from the opponent across pods using Redis Pub/Sub.
func (h *RedisLiveSyncHub) SubscribeMatchUpdates(ctx context.Context, matchID, myUserID string) (<-chan port.ParticipantProgress, func(), error) {
	streamKey := "solo_match:" + matchID + ":stream"
	pubsub := h.client.Subscribe(ctx, streamKey)

	// Verify subscription
	if _, err := pubsub.Receive(ctx); err != nil {
		if closeErr := pubsub.Close(); closeErr != nil {
			slog.WarnContext(ctx, "failed to close pubsub on subscription failure", "err", closeErr)
		}
		return nil, nil, fmt.Errorf("redis subscribe match stream: %w", err)
	}

	ch := make(chan port.ParticipantProgress, 100)
	subCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(ch)

		redisCh := pubsub.Channel()
		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-redisCh:
				if !ok {
					return
				}
				var prog port.ParticipantProgress
				if err := json.Unmarshal([]byte(msg.Payload), &prog); err != nil {
					continue
				}
				if prog.UserID == myUserID {
					continue
				}
				select {
				case ch <- prog:
				default:
					// Non-blocking drop to keep real-time latency minimal
				}
			}
		}
	}()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			cancel()
			if closeErr := pubsub.Close(); closeErr != nil {
				slog.Warn("failed to close pubsub on unsubscribe", "err", closeErr)
			}
		})
	}

	return ch, unsubscribe, nil
}

// DelRoom explicitly removes all Redis room keys (participants, progress, exercise) for a match.
func (h *RedisLiveSyncHub) DelRoom(ctx context.Context, matchID string) error {
	partKey := "solo_match:" + matchID + ":participants"
	progKey := "solo_match:" + matchID + ":progress"
	metaKey := "solo_match:" + matchID + ":exercise"
	return h.client.Del(ctx, partKey, progKey, metaKey).Err()
}
