package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

type cloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype,omitempty"`
	Data            json.RawMessage `json:"data"`
}

type soloTrackingCompletedData struct {
	MatchID        string `json:"matchId"`
	UserID         string `json:"userId"`
	ConfirmedValue int32  `json:"confirmedValue"`
	CompletedAt    string `json:"completedAt,omitempty"`
}

type WorkoutCompletedConsumer struct {
	handler       *command.RecordParticipantResultHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewWorkoutCompletedConsumer(
	handler *command.RecordParticipantResultHandler,
	outboxLogRepo port.OutboxLogRepository,
) *WorkoutCompletedConsumer {
	return &WorkoutCompletedConsumer{
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func isSoloTrackingCompletedEvent(eventType string) bool {
	return strings.HasSuffix(eventType, "soloTrackingCompleted") ||
		strings.HasSuffix(eventType, "SoloTrackingCompleted") ||
		eventType == "contracts.core.workout_execution.v1.soloTrackingCompleted"
}

func (c *WorkoutCompletedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("workout completed consumer: unmarshal cloudevent: %w", err)
	}

	if !isSoloTrackingCompletedEvent(env.Type) {
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

	var data soloTrackingCompletedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("workout completed consumer: unmarshal data: %w", err)
	}

	// Bỏ qua nếu event không thuộc về trận đấu nào hoặc thiếu người dùng
	if data.MatchID == "" || data.UserID == "" {
		return nil
	}

	completedAt := time.Now().UTC()
	if data.CompletedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, data.CompletedAt); err == nil {
			completedAt = parsed
		}
	}

	log.Printf("[SoloMatch] Received %s for match %s, user %s, confirmedValue %d", env.Type, data.MatchID, data.UserID, data.ConfirmedValue)

	err := c.handler.Handle(ctx, command.RecordParticipantResultCommand{
		MatchID:     data.MatchID,
		UserID:      data.UserID,
		ScoreValue:  float64(data.ConfirmedValue),
		CompletedAt: completedAt,
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
