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
	ID      string          `json:"id"`
	Source  string          `json:"source"`
	Type    string          `json:"type"`
	Time    string          `json:"time"`
	MatchID string          `json:"matchId,omitempty"`
	Data    json.RawMessage `json:"data"`
}

type workoutCompletedData struct {
	SessionID   string  `json:"sessionId"`
	UserID      string  `json:"userId"`
	MatchID     string  `json:"matchId,omitempty"`
	ScoreValue  float64 `json:"scoreValue,omitempty"`
	TotalSets   int32   `json:"totalSets,omitempty"`
	TotalVolume float64 `json:"totalVolume,omitempty"`
	PlanID      string  `json:"planId,omitempty"`
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

func (c *WorkoutCompletedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("workout completed consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "workoutSessionCompleted") &&
		env.Type != "contracts.core.workout_execution.v1.workoutSessionCompleted" {
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

	var data workoutCompletedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("workout completed consumer: unmarshal data: %w", err)
	}

	matchID := data.MatchID
	if matchID == "" {
		matchID = env.MatchID
	}
	// Nếu không có matchId, đây là buổi tập thông thường, bỏ qua
	if matchID == "" {
		return nil
	}

	score := data.ScoreValue
	if score <= 0 {
		score = float64(data.TotalSets)
	}

	log.Printf("[SoloMatch] Received WorkoutSessionCompleted for match %s, user %s, score %v", matchID, data.UserID, score)

	err := c.handler.Handle(ctx, command.RecordParticipantResultCommand{
		MatchID:     matchID,
		UserID:      data.UserID,
		ScoreValue:  score,
		CompletedAt: time.Now().UTC(),
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
