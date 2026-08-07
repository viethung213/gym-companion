package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
)

// workoutSessionCompletedPayload maps the inner payload for WorkoutSessionCompleted event.
type workoutSessionCompletedPayload struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
}

// PREventConsumer listens for completed sessions and triggers asynchronous 1RM Personal Record calculation.
type PREventConsumer struct {
	prHandler *command.ProcessCompletedSessionForPRHandler
}

// NewPREventConsumer constructs PREventConsumer.
func NewPREventConsumer(prHandler *command.ProcessCompletedSessionForPRHandler) *PREventConsumer {
	return &PREventConsumer{
		prHandler: prHandler,
	}
}

// HandleMessage parses a CloudEvents JSON message from Kafka and triggers OnWorkoutSessionCompleted.
func (c *PREventConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("pr event consumer: unmarshal cloudevent: %w", err)
	}

	// Filter out non-WorkoutSessionCompleted events
	if !strings.HasSuffix(env.Type, "workoutSessionCompleted") && env.Type != "contracts.core.workout_execution.v1.workoutSessionCompleted" {
		return nil
	}

	var data workoutSessionCompletedPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("pr event consumer: unmarshal data payload: %w", err)
	}

	if data.SessionID == "" || data.UserID == "" {
		return fmt.Errorf("pr event consumer: empty sessionId or userId in event payload")
	}

	return c.OnWorkoutSessionCompleted(ctx, data.SessionID, data.UserID)
}

// OnWorkoutSessionCompleted processes session completion event to calculate 1RM.
func (c *PREventConsumer) OnWorkoutSessionCompleted(ctx context.Context, sessionID, userID string) error {
	log.Printf("[WorkoutExecution] Asynchronously evaluating 1RM PR for session %s (user %s)", sessionID, userID)
	return c.prHandler.HandleProcess(ctx, sessionID, userID)
}
