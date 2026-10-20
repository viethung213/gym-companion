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

type workoutAbortedData struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	MatchID   string `json:"matchId,omitempty"`
	Reason    string `json:"reason,omitempty"`
	PlanID    string `json:"planId,omitempty"`
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

func (c *WorkoutAbortedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("workout aborted consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "workoutSessionAborted") &&
		env.Type != "contracts.core.workout_execution.v1.workoutSessionAborted" {
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

	var data workoutAbortedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("workout aborted consumer: unmarshal data: %w", err)
	}

	matchID := data.MatchID
	if matchID == "" {
		matchID = env.MatchID
	}
	if matchID == "" {
		return nil
	}

	reason := data.Reason
	if reason == "" {
		reason = "workout_session_aborted"
	}

	log.Printf("[SoloMatch] Received WorkoutSessionAborted for match %s, user %s, reason: %s", matchID, data.UserID, reason)

	err := c.handler.Handle(ctx, command.RecordParticipantAbortCommand{
		MatchID: matchID,
		UserID:  data.UserID,
		Reason:  reason,
	})

	if c.outboxLogRepo != nil && env.ID != "" {
		if err != nil {
			_ = c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, "FAILED", err.Error())
		} else {
			_ = c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, "PROCESSED", "")
		}
	}

	return err
}
