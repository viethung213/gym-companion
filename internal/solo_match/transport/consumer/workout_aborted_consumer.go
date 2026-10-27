package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

type soloTrackingAbortedData struct {
	MatchID string `json:"matchId"`
	UserID  string `json:"userId"`
	Reason  string `json:"reason,omitempty"`
}

type WorkoutAbortedConsumer struct {
	handler       *command.RecordParticipantAbortHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewWorkoutAbortedConsumer(
	handler *command.RecordParticipantAbortHandler,
	outboxLogRepo port.OutboxLogRepository,
) *WorkoutAbortedConsumer {
	return &WorkoutAbortedConsumer{
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func isSoloTrackingAbortedEvent(eventType string) bool {
	return strings.HasSuffix(eventType, "soloTrackingAborted") ||
		strings.HasSuffix(eventType, "SoloTrackingAborted") ||
		eventType == "contracts.core.workout_execution.v1.soloTrackingAborted"
}

func (c *WorkoutAbortedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("workout aborted consumer: unmarshal cloudevent: %w", err)
	}

	if !isSoloTrackingAbortedEvent(env.Type) {
		return nil
	}

	// Idempotency check: Bỏ qua nếu event này đã được xử lý thành công trước đó
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[SoloMatch] OutboxLog IsProcessed error: %v", err)
		} else if processed {
			log.Printf("[SoloMatch] Event %s already processed, skipping (Idempotent)", env.ID)
			return nil
		}
	}

	var data soloTrackingAbortedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("workout aborted consumer: unmarshal data: %w", err)
	}

	// Bỏ qua nếu event không thuộc về trận đấu nào hoặc thiếu người dùng
	if data.MatchID == "" || data.UserID == "" {
		return nil
	}

	reason := data.Reason
	if reason == "" {
		reason = "USER_FORFEITED"
	}

	log.Printf("[SoloMatch] Received %s for match %s, user %s, reason: %s", env.Type, data.MatchID, data.UserID, reason)

	err := c.handler.Handle(ctx, command.RecordParticipantAbortCommand{
		MatchID: data.MatchID,
		UserID:  data.UserID,
		Reason:  reason,
	})

	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		if saveErr := c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, status, errMsg); saveErr != nil {
			log.Printf("[SoloMatch] Failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return err
}
