package worker

import (
	"context"
	"log"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
)

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

// OnWorkoutSessionCompleted processes session completion event to calculate 1RM.
func (c *PREventConsumer) OnWorkoutSessionCompleted(ctx context.Context, sessionID, userID string) error {
	log.Printf("[WorkoutExecution] Asynchronously evaluating 1RM PR for session %s (user %s)", sessionID, userID)
	return c.prHandler.HandleProcess(ctx, sessionID, userID)
}
