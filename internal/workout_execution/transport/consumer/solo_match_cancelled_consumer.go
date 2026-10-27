package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

type soloMatchCancelledData struct {
	MatchID string `json:"matchId"`
	Reason  string `json:"reason,omitempty"`
}

// BotCanceler defines interface to stop a running bot goroutine for a given match.
type BotCanceler interface {
	CancelBot(matchID string)
}

// SoloMatchCancelledConsumer listens to SoloMatchCancelled events from solo_match module
// and performs cleanup: cancels bot runner and deletes Redis room keys.
type SoloMatchCancelledConsumer struct {
	liveSync      port.SoloMatchLiveSyncPort
	botCanceler   BotCanceler
	outboxLogRepo port.OutboxLogRepository
}

func NewSoloMatchCancelledConsumer(
	liveSync port.SoloMatchLiveSyncPort,
	botCanceler BotCanceler,
	outboxLogRepo port.OutboxLogRepository,
) *SoloMatchCancelledConsumer {
	return &SoloMatchCancelledConsumer{
		liveSync:      liveSync,
		botCanceler:   botCanceler,
		outboxLogRepo: outboxLogRepo,
	}
}

func isSoloMatchCancelledEvent(eventType string) bool {
	return strings.HasSuffix(eventType, "soloMatchCancelled") ||
		strings.HasSuffix(eventType, "SoloMatchCancelled") ||
		eventType == "contracts.core.solo_match.v1.soloMatchCancelled"
}

// HandleMessage parses SoloMatchCancelled CloudEvent and performs immediate room and bot cleanup.
func (c *SoloMatchCancelledConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("solo match cancelled consumer: unmarshal cloudevent: %w", err)
	}

	if !isSoloMatchCancelledEvent(env.Type) {
		return nil
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[WorkoutExecution] Warning checking outbox_log idempotency for %s: %v", env.ID, err)
		} else if processed {
			return nil
		}
	}

	var data soloMatchCancelledData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("solo match cancelled consumer: unmarshal data payload: %w", err)
	}

	if data.MatchID == "" {
		return nil
	}

	log.Printf("[WorkoutExecution] Received SoloMatchCancelled for match %s (reason: %s). Purging room and canceling bot...", data.MatchID, data.Reason)

	// 1. Broadcast thông báo CANCELLED vào phòng Redis để client đang kết nối gRPC stream nhận được
	if c.liveSync != nil {
		cancelProg := port.ParticipantProgress{
			UserID: "SYSTEM",
			Status: "CANCELLED",
		}
		if pubErr := c.liveSync.PublishMyProgress(ctx, data.MatchID, cancelProg); pubErr != nil {
			log.Printf("[WorkoutExecution] Warning: failed to broadcast cancel progress for match %s: %v", data.MatchID, pubErr)
		}
	}

	// 2. Hủy bot runner nếu có
	if c.botCanceler != nil {
		c.botCanceler.CancelBot(data.MatchID)
	}

	// 3. Xóa sạch phòng trên Redis (chờ 100ms để thông báo pub/sub kịp chuyển tới client)
	if c.liveSync != nil {
		time.Sleep(100 * time.Millisecond)
		if delErr := c.liveSync.DelRoom(ctx, data.MatchID); delErr != nil {
			log.Printf("[WorkoutExecution] Warning: failed to del Redis room for cancelled match %s: %v", data.MatchID, delErr)
		}
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		logRecord := &port.OutboxLogRecord{
			ID:           env.ID,
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      rawPayload,
			PartitionKey: data.MatchID,
			Status:       "PROCESSED",
		}
		if saveErr := c.outboxLogRepo.SaveLog(ctx, logRecord); saveErr != nil {
			log.Printf("[WorkoutExecution] Warning: failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return nil
}
