package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

type soloMatchStartedPayload struct {
	MatchID       string `json:"matchId"`
	ExerciseID    string `json:"exerciseId"`
	ExerciseName  string `json:"exerciseName"`
	ChallengeType string `json:"challengeType"`
	Player1ID     string `json:"player1Id"`
	Player2ID     string `json:"player2Id"`
	BotUserID     string `json:"botUserId"`
}

// SoloMatchStartedConsumer listens to SoloMatchStarted events and delegates bot execution to SoloBotManager.
type SoloMatchStartedConsumer struct {
	botManager    port.SoloBotManager
	outboxLogRepo port.OutboxLogRepository
}

// NewSoloMatchStartedConsumer constructs a new SoloMatchStartedConsumer.
func NewSoloMatchStartedConsumer(
	botManager port.SoloBotManager,
	outboxLogRepo port.OutboxLogRepository,
) *SoloMatchStartedConsumer {
	return &SoloMatchStartedConsumer{
		botManager:    botManager,
		outboxLogRepo: outboxLogRepo,
	}
}

// HandleMessage parses a CloudEvents JSON message and triggers bot simulation if BotUserID is present.
func (c *SoloMatchStartedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("solo match started consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "soloMatchStarted") && env.Type != "contracts.core.solo_match.v1.soloMatchStarted" {
		return nil
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[WorkoutExecution] Warning checking outbox_log idempotency for %s: %v", env.ID, err)
		} else if processed {
			log.Printf("[WorkoutExecution] SoloMatchStarted event %s already processed, skipping", env.ID)
			return nil
		}
	}

	var data soloMatchStartedPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("solo match started consumer: unmarshal data payload: %w", err)
	}

	var handleErr error
	if data.BotUserID != "" && c.botManager != nil {
		userPlayerID := data.Player1ID
		if userPlayerID == data.BotUserID {
			userPlayerID = data.Player2ID
		}
		handleErr = c.botManager.StartBot(
			ctx,
			data.MatchID,
			data.BotUserID,
			userPlayerID,
			data.ExerciseID,
			data.ChallengeType,
		)
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if handleErr != nil {
			status = "FAILED"
			errMsg = handleErr.Error()
		}

		partitionKey := data.MatchID
		if data.BotUserID != "" {
			partitionKey = data.BotUserID
		}

		logRecord := &port.OutboxLogRecord{
			ID:           env.ID,
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      rawPayload,
			PartitionKey: partitionKey,
			Status:       status,
			ErrorMessage: errMsg,
		}
		if saveErr := c.outboxLogRepo.SaveLog(ctx, logRecord); saveErr != nil {
			log.Printf("[WorkoutExecution] Warning: failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return handleErr
}
