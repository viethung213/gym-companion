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
)

type NutritionPayload struct {
	UserID       string `json:"userId"`
	ActivityDate string `json:"activityDate"`
}

type NutritionActivityHandler interface {
	Handle(ctx context.Context, cmd command.ProcessNutritionActivityCommand) error
}

type NutritionEventConsumer struct {
	reader        *kafka.Reader
	handler       NutritionActivityHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewNutritionEventConsumer(
	reader *kafka.Reader,
	handler NutritionActivityHandler,
	outboxLogRepo port.OutboxLogRepository,
) *NutritionEventConsumer {
	return &NutritionEventConsumer{
		reader:        reader,
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *NutritionEventConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting NutritionEvent Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping NutritionEvent Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] NutritionEvent consumer fetch error: %v", err)
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
				log.Printf("[Gamification] NutritionEvent consumer process error after retries: %v", processErr)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] NutritionEvent consumer commit error: %v", err)
			}
		}
	}
}

func (c *NutritionEventConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}

	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !strings.Contains(ce.Type, "Nutrition") &&
		!strings.Contains(ce.Type, "nutrition") {
		return nil
	}

	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if err == nil && processed {
			return nil
		}
	}

	var payload NutritionPayload
	if err := json.Unmarshal(ce.Data, &payload); err != nil {
		return fmt.Errorf("unmarshal nutrition payload: %w", err)
	}

	userID := payload.UserID
	if userID == "" {
		userID = string(msg.Key)
	}
	activityDate := payload.ActivityDate
	if activityDate == "" {
		activityDate = time.Now().UTC().Format("2006-01-02")
	}

	cmd := command.ProcessNutritionActivityCommand{
		UserID:       userID,
		ActivityDate: activityDate,
		RecordedAt:   time.Now().UTC(),
	}

	var processErr error
	if c.handler != nil {
		processErr = c.handler.Handle(ctx, cmd)
	}

	if processErr != nil {
		if c.outboxLogRepo != nil && ce.ID != "" {
			_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      ce.ID,
				EventType:    ce.Type,
				Payload:      msg.Value,
				PartitionKey: string(msg.Key),
				Status:       "FAILED",
				ErrorMessage: processErr.Error(),
			})
		}
		return processErr
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
