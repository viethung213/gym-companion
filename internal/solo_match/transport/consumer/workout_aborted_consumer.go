package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
			slog.WarnContext(ctx, "OutboxLog IsProcessed error", slog.String("event_id", env.ID), slog.String("error", err.Error()))
		} else if processed {
			slog.DebugContext(ctx, "Event already processed, skipping (Idempotent)", slog.String("event_id", env.ID))
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

	slog.InfoContext(ctx, "Received soloTrackingAborted event",
		slog.String("event_type", env.Type),
		slog.String("match_id", data.MatchID),
		slog.String("user_id", data.UserID),
		slog.String("reason", reason),
	)

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
			slog.WarnContext(ctx, "Failed to save outbox log for event", slog.String("event_id", env.ID), slog.String("error", saveErr.Error()))
		}
	}

	return err
}
