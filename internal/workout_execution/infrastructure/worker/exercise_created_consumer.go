package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// cloudEventEnvelope maps the outer CloudEvents 1.0 JSON payload structure.
type cloudEventEnvelope struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// exerciseCreatedPayload maps the inner payload for ExerciseCreated event.
type exerciseCreatedPayload struct {
	ExerciseID string `json:"exerciseId"`
}

// ExerciseCreatedConsumer listens for ExerciseCreated events from the Exercise domain
// and initializes an empty/draft MotionSpecification for the exercise.
type ExerciseCreatedConsumer struct {
	motionRepo    repository.MotionSpecificationRepository
	outboxLogRepo port.OutboxLogRepository
}

// NewExerciseCreatedConsumer constructs ExerciseCreatedConsumer.
func NewExerciseCreatedConsumer(
	motionRepo repository.MotionSpecificationRepository,
	outboxLogRepo port.OutboxLogRepository,
) *ExerciseCreatedConsumer {
	return &ExerciseCreatedConsumer{
		motionRepo:    motionRepo,
		outboxLogRepo: outboxLogRepo,
	}
}

// HandleMessage parses a CloudEvents JSON message from Kafka and triggers OnExerciseCreated.
func (c *ExerciseCreatedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("exercise created consumer: unmarshal cloudevent: %w", err)
	}

	// Filter out non-ExerciseCreated events
	if !strings.HasSuffix(env.Type, "exerciseCreated") && env.Type != "contracts.supporting.exercise.v1.exerciseCreated" {
		return nil
	}

	// Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[WorkoutExecution] Warning checking outbox_log idempotency: %v", err)
		} else if processed {
			log.Printf("[WorkoutExecution] Event %s already processed in outbox_log, skipping", env.ID)
			return nil
		}
	}

	var data exerciseCreatedPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("exercise created consumer: unmarshal data payload: %w", err)
	}

	if data.ExerciseID == "" {
		return fmt.Errorf("exercise created consumer: empty exerciseId in event payload")
	}

	log.Printf("[WorkoutExecution] Received ExerciseCreated event for exercise_id: %s (event_type: %s)", data.ExerciseID, env.Type)
	handleErr := c.OnExerciseCreated(ctx, data.ExerciseID)

	// Record processed/failed event in outbox_log for idempotency and auditing
	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if handleErr != nil {
			status = "FAILED"
			errMsg = handleErr.Error()
		}

		logRecord := &port.OutboxLogRecord{
			ID:           env.ID,
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      rawPayload,
			PartitionKey: data.ExerciseID,
			Status:       status,
			ErrorMessage: errMsg,
		}
		if logErr := c.outboxLogRepo.SaveLog(ctx, logRecord); logErr != nil {
			log.Printf("[WorkoutExecution] Failed to save outbox_log record for event %s: %v", env.ID, logErr)
		}
	}

	return handleErr
}

// OnExerciseCreated handles creating a draft MotionSpecification for a newly created exercise.
func (c *ExerciseCreatedConsumer) OnExerciseCreated(ctx context.Context, exerciseID string) error {
	if exerciseID == "" {
		return fmt.Errorf("exercise created consumer: exercise_id is required")
	}

	existing, err := c.motionRepo.FindByExerciseID(ctx, exerciseID)
	if err == nil && existing != nil {
		log.Printf("[WorkoutExecution] Draft MotionSpecification already exists for exercise_id: %s, skipping", exerciseID)
		return nil
	}
	if err != nil && !errors.Is(err, derror.ErrNotFound) {
		return fmt.Errorf("exercise created consumer check existing: %w", err)
	}

	defaultDetectorURL := os.Getenv("DEFAULT_ONNX_DETECTOR_URL")
	defaultSkeletonURL := os.Getenv("DEFAULT_ONNX_SKELETON_URL")
	draft := aggregate.NewDraftMotionSpecification(exerciseID, defaultDetectorURL, defaultSkeletonURL)
	if err := c.motionRepo.Save(ctx, draft); err != nil {
		return fmt.Errorf("exercise created consumer save draft: %w", err)
	}

	log.Printf("[WorkoutExecution] Successfully created draft MotionSpecification for exercise_id: %s", exerciseID)
	return nil
}
