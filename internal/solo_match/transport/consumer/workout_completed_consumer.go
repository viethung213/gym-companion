package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
			slog.WarnContext(ctx, "OutboxLog IsProcessed error", slog.String("event_id", env.ID), slog.String("error", err.Error()))
		} else if processed {
			slog.DebugContext(ctx, "Event already processed, skipping (Idempotent)", slog.String("event_id", env.ID))
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

	slog.InfoContext(ctx, "Received soloTrackingCompleted event",
		slog.String("event_type", env.Type),
		slog.String("match_id", data.MatchID),
		slog.String("user_id", data.UserID),
		slog.Int("confirmed_value", int(data.ConfirmedValue)),
	)

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
			slog.WarnContext(ctx, "Failed to save outbox log for event", slog.String("event_id", env.ID), slog.String("error", saveErr.Error()))
		}
	}

	return err
}
