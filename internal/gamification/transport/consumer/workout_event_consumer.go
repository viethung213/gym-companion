package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

// WorkoutEventConsumer listens to completed workout sessions and coordinates gamification progression.
type WorkoutEventConsumer struct {
	reader        *kafka.Reader
	coordinator   *appService.ActivityIngestionCoordinator
	outboxLogRepo port.OutboxLogRepository
}

// NewWorkoutEventConsumer constructs WorkoutEventConsumer.
func NewWorkoutEventConsumer(
	reader *kafka.Reader,
	coordinator *appService.ActivityIngestionCoordinator,
	outboxLogRepo port.OutboxLogRepository,
) *WorkoutEventConsumer {
	return &WorkoutEventConsumer{
		reader:        reader,
		coordinator:   coordinator,
		outboxLogRepo: outboxLogRepo,
	}
}

// Start polls Kafka for completed workout session events until context cancellation.
func (c *WorkoutEventConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting WorkoutEvent Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping WorkoutEvent Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] WorkoutEvent consumer fetch error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = c.ProcessMessage(ctx, &msg)
				if processErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
			}

			if processErr != nil {
				log.Printf("[Gamification] WorkoutEvent consumer process error after retries: %v", processErr)
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] WorkoutEvent consumer commit error: %v", err)
			}
		}
	}
}

// ProcessMessage deserializes event, verifies idempotency, and delegates to coordinator.
func (c *WorkoutEventConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}
	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !strings.Contains(ce.Type, "workoutSessionCompleted") &&
		!strings.Contains(ce.Type, "WorkoutSessionCompleted") {
		return nil
	}

	// Idempotency check via outbox_log
	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if err == nil && processed {
			return nil
		}
	}

	var protoEv workoutexecutionv1event.WorkoutSessionCompleted
	if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
		return fmt.Errorf("unmarshal proto WorkoutSessionCompleted: %w", err)
	}

	completedAt := time.Now().UTC()
	if protoEv.CompletedAt != nil {
		completedAt = protoEv.CompletedAt.AsTime()
	}
	activityDate := completedAt.UTC().Format("2006-01-02")

	cmd := &command.ProcessWorkoutXpCommand{
		UserID:           protoEv.GetUserId(),
		SessionID:        protoEv.GetSessionId(),
		ActivityDate:     activityDate,
		TotalSets:        protoEv.GetTotalSets(),
		TotalVolumeKg:    float64(protoEv.GetTotalVolume()),
		AverageFormScore: protoEv.AverageFormScore,
		HasNewPR:         false,
		CompletedAt:      completedAt,
	}

	err := c.coordinator.IngestWorkoutSession(ctx, cmd)
	if err != nil {
		if c.outboxLogRepo != nil && ce.ID != "" {
			_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      ce.ID,
				EventType:    ce.Type,
				Payload:      msg.Value,
				PartitionKey: string(msg.Key),
				Status:       "FAILED",
				ErrorMessage: err.Error(),
			})
		}
		return err
	}

	if c.outboxLogRepo != nil && ce.ID != "" {
		_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      ce.ID,
			EventType:    ce.Type,
			Payload:      msg.Value,
			PartitionKey: string(msg.Key),
			Status:       "PROCESSED",
		})
	}

	return nil
}
