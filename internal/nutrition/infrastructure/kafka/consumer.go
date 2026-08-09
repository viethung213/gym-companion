package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
)

type cloudEventEnvelope struct {
	SpecVersion string          `json:"specversion"`
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type WorkoutSessionCompletedPayload struct {
	SessionID           string  `json:"sessionId"`
	UserID              string  `json:"userId"`
	TotalCaloriesBurned float64 `json:"totalCaloriesBurned"`
	TotalVolume         float64 `json:"totalVolume"`
	CompletedAt         string  `json:"completedAt"`
}

type Consumer struct {
	reader             *segmentio.Reader
	recalibrateHandler *command.RecalibratePlanWithPantryHandler
	outboxLogRepo      port.OutboxLogRepository
}

func NewConsumer(
	reader *segmentio.Reader,
	recalibrateHandler *command.RecalibratePlanWithPantryHandler,
	outboxLogRepo port.OutboxLogRepository,
) *Consumer {
	return &Consumer{
		reader:             reader,
		recalibrateHandler: recalibrateHandler,
		outboxLogRepo:      outboxLogRepo,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	if c.reader == nil {
		log.Println("[Nutrition Kafka Consumer] Kafka reader is nil, skipping consumer loop.")
		return nil
	}

	log.Println("[Nutrition Kafka Consumer] Started listening to Kafka topic 'workout_execution.events'")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Nutrition Kafka Consumer] Shutting down Kafka consumer.")
			return nil
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("[Nutrition Kafka Consumer] Error fetching message: %v", err)
				continue
			}

			c.handleMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Nutrition Kafka Consumer] Error committing message: %v", err)
			}
		}
	}
}

//nolint:gocritic // msg is passed by value per kafka library signature
func (c *Consumer) handleMessage(ctx context.Context, msg segmentio.Message) {
	var env cloudEventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		log.Printf("[Nutrition Kafka Consumer] Error unmarshaling CloudEvent envelope: %v", err)
		return
	}

	if !strings.HasSuffix(env.Type, "workoutSessionCompleted") && !strings.HasSuffix(env.Type, "WorkoutSessionCompleted") {
		return
	}

	// 1. Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[Nutrition Kafka Consumer] Warning checking outbox log idempotency: %v", err)
		} else if processed {
			log.Printf("[Nutrition Kafka Consumer] Event %s already processed in outbox_log, skipping", env.ID)
			return
		}
	}

	var payload WorkoutSessionCompletedPayload
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			log.Printf("[Nutrition Kafka Consumer] Error unmarshaling event payload: %v", err)
			c.saveLog(ctx, msg.Value, env, "", err)
			return
		}
	} else {
		// Fallback if payload is not wrapped in data
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			log.Printf("[Nutrition Kafka Consumer] Error unmarshaling raw payload: %v", err)
			c.saveLog(ctx, msg.Value, env, "", err)
			return
		}
	}

	log.Printf("[Nutrition Kafka Consumer] Received WorkoutSessionCompleted Event: UserID=%s, SessionID=%s, Volume=%.2f",
		payload.UserID, payload.SessionID, payload.TotalVolume)

	var processErr error
	if c.recalibrateHandler != nil && payload.UserID != "" {
		_, processErr = c.recalibrateHandler.Handle(ctx, command.RecalibratePlanWithPantryCommand{
			UserID:               payload.UserID,
			PlanDate:             time.Now(),
			AvailableIngredients: nil,
		})
		if processErr != nil {
			log.Printf("[Nutrition Kafka Consumer] Failed to recalibrate plan on workout event: %v", processErr)
		} else {
			log.Printf("[Nutrition Kafka Consumer] Successfully rebalanced nutrition plan for user %s", payload.UserID)
		}
	}

	c.saveLog(ctx, msg.Value, env, payload.UserID, processErr)
}

//nolint:gocritic // env envelope value object is passed by value per helper design
func (c *Consumer) saveLog(ctx context.Context, rawPayload []byte, env cloudEventEnvelope, userID string, err error) {
	if c.outboxLogRepo == nil || env.ID == "" {
		return
	}
	status := "PROCESSED"
	errMsg := ""
	if err != nil {
		status = "FAILED"
		errMsg = err.Error()
	}
	payload := rawPayload
	if len(payload) == 0 {
		payload = env.Data
	}
	logRecord := &port.OutboxLogRecord{
		ID:           env.ID,
		EventID:      env.ID,
		EventType:    env.Type,
		Payload:      payload,
		PartitionKey: userID,
		Status:       status,
		ErrorMessage: errMsg,
	}
	if saveErr := c.outboxLogRepo.SaveLog(ctx, logRecord); saveErr != nil {
		log.Printf("[Nutrition Kafka Consumer] Warning: failed to save outbox log for event %s: %v", env.ID, saveErr)
	}
}
