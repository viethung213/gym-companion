package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
			slog.WarnContext(ctx, "Warning checking outbox_log idempotency", slog.String("event_id", env.ID), slog.String("error", err.Error()))
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

	slog.InfoContext(ctx, "Received SoloMatchCancelled, purging room and canceling bot",
		slog.String("match_id", data.MatchID),
		slog.String("reason", data.Reason),
	)

	// 1. Broadcast thông báo CANCELLED vào phòng Redis để client đang kết nối gRPC stream nhận được
	if c.liveSync != nil {
		cancelProg := port.ParticipantProgress{
			UserID: "SYSTEM",
			Status: "CANCELLED",
		}
		if pubErr := c.liveSync.PublishMyProgress(ctx, data.MatchID, cancelProg); pubErr != nil {
			slog.WarnContext(ctx, "Failed to broadcast cancel progress for match", slog.String("match_id", data.MatchID), slog.String("error", pubErr.Error()))
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
			slog.WarnContext(ctx, "Failed to del Redis room for cancelled match", slog.String("match_id", data.MatchID), slog.String("error", delErr.Error()))
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
			slog.WarnContext(ctx, "Failed to save outbox log for event", slog.String("event_id", env.ID), slog.String("error", saveErr.Error()))
		}
	}

	return nil
}
