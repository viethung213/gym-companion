package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"google.golang.org/protobuf/encoding/protojson"
)

type WorkoutSharedConsumer struct {
	reader        *kafka.Reader
	ingestHandler *command.IngestWorkoutActivityHandler
	outboxLogRepo port.OutboxLogRepository
	txManager     port.TransactionManager
}

func NewWorkoutSharedConsumer(
	reader *kafka.Reader,
	ingestHandler *command.IngestWorkoutActivityHandler,
	outboxLogRepo port.OutboxLogRepository,
	txManager port.TransactionManager,
) *WorkoutSharedConsumer {
	return &WorkoutSharedConsumer{
		reader:        reader,
		ingestHandler: ingestHandler,
		outboxLogRepo: outboxLogRepo,
		txManager:     txManager,
	}
}

func (c *WorkoutSharedConsumer) Start(ctx context.Context) {
	log.Println("Starting Social WorkoutShared Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social WorkoutShared Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Social WorkoutShared consumer read error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("Social WorkoutShared consumer process error: %v", err)
			}
		}
	}
}

//nolint:gocritic // kafka.Message is passed by value matching consumer handler contract
func (c *WorkoutSharedConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var cloudEvent CloudEvent
	if err := json.Unmarshal(msg.Value, &cloudEvent); err != nil {
		return fmt.Errorf("unmarshal cloud event envelope: %w", err)
	}

	if cloudEvent.Type != "contracts.core.workout_execution.v1.workoutSessionShared" {
		return nil
	}

	// Idempotency check
	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, cloudEvent.ID)
		if err == nil && processed {
			return nil
		}
	}

	var event workoutexecutionv1event.WorkoutSessionShared
	if err := protojson.Unmarshal(cloudEvent.Data, &event); err != nil {
		return fmt.Errorf("unmarshal WorkoutSessionShared data: %w", err)
	}

	err := c.handleEvent(ctx, cloudEvent.ID, &event)
	if err != nil {
		if c.outboxLogRepo != nil && cloudEvent.ID != "" {
			_ = c.outboxLogRepo.Save(ctx, cloudEvent.ID, cloudEvent.Type, msg.Value, string(msg.Key), "FAILED", err.Error())
		}
		return err
	}

	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		_ = c.outboxLogRepo.Save(ctx, cloudEvent.ID, cloudEvent.Type, msg.Value, string(msg.Key), "PROCESSED", "")
	}

	return nil
}

func (c *WorkoutSharedConsumer) handleEvent(ctx context.Context, _ string, event *workoutexecutionv1event.WorkoutSessionShared) error {
	sharedAt := time.Now().UTC()
	if event.GetSharedAt() != nil {
		sharedAt = event.GetSharedAt().AsTime()
	}

	return c.ingestHandler.Handle(ctx, command.IngestWorkoutActivityCommand{
		SessionID:       event.GetSessionId(),
		UserID:          event.GetUserId(),
		Title:           event.GetTitle(),
		Caption:         event.GetCaption(),
		MediaURLs:       event.GetMediaUrls(),
		DurationSeconds: event.GetDurationSeconds(),
		TotalVolumeKg:   float64(event.GetTotalVolumeKg()),
		TotalSets:       event.GetTotalSets(),
		PRCount:         event.GetPrCount(),
		SharedAt:        sharedAt,
	})
}
