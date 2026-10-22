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
	workoutexecutionv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type PersonalRecordHandler interface {
	Handle(ctx context.Context, cmd command.ProcessPersonalRecordCommand) error
}

type PersonalRecordConsumer struct {
	reader        *kafka.Reader
	handler       PersonalRecordHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewPersonalRecordConsumer(
	reader *kafka.Reader,
	handler PersonalRecordHandler,
	outboxLogRepo port.OutboxLogRepository,
) *PersonalRecordConsumer {
	return &PersonalRecordConsumer{
		reader:        reader,
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *PersonalRecordConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting PersonalRecord Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping PersonalRecord Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] PersonalRecord consumer fetch error: %v", err)
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
				log.Printf("[Gamification] PersonalRecord consumer process error after retries: %v", processErr)
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] PersonalRecord consumer commit error: %v", err)
			}
		}
	}
}

func (c *PersonalRecordConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}

	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !strings.Contains(ce.Type, "newPersonalRecordAchieved") &&
		!strings.Contains(ce.Type, "NewPersonalRecordAchieved") {
		return nil
	}

	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, pErr := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if pErr == nil && processed {
			return nil
		}
	}

	var protoEv workoutexecutionv1event.NewPersonalRecordAchieved
	if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
		return fmt.Errorf("unmarshal proto NewPersonalRecordAchieved: %w", err)
	}

	achievedAt := time.Now().UTC()
	if protoEv.AchievedAt != nil {
		achievedAt = protoEv.AchievedAt.AsTime()
	}

	cmd := command.ProcessPersonalRecordCommand{
		UserID:       protoEv.GetUserId(),
		ExerciseID:   protoEv.GetExerciseId(),
		Weight:       float64(protoEv.GetWeight()),
		Reps:         protoEv.GetReps(),
		ActivityDate: achievedAt.UTC().Format("2006-01-02"),
		AchievedAt:   achievedAt,
	}

	var err error
	if c.handler != nil {
		err = c.handler.Handle(ctx, cmd)
	}

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
