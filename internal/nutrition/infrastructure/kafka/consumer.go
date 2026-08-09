package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
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
}

func NewConsumer(reader *segmentio.Reader, recalibrateHandler *command.RecalibratePlanWithPantryHandler) *Consumer {
	return &Consumer{
		reader:             reader,
		recalibrateHandler: recalibrateHandler,
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

	var payload WorkoutSessionCompletedPayload
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			log.Printf("[Nutrition Kafka Consumer] Error unmarshaling event payload: %v", err)
			return
		}
	} else {
		// Fallback if payload is not wrapped in data
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			log.Printf("[Nutrition Kafka Consumer] Error unmarshaling raw payload: %v", err)
			return
		}
	}

	log.Printf("[Nutrition Kafka Consumer] Received WorkoutSessionCompleted Event: UserID=%s, SessionID=%s, Volume=%.2f",
		payload.UserID, payload.SessionID, payload.TotalVolume)

	if c.recalibrateHandler != nil && payload.UserID != "" {
		_, err := c.recalibrateHandler.Handle(ctx, command.RecalibratePlanWithPantryCommand{
			UserID:               payload.UserID,
			PlanDate:             time.Now(),
			AvailableIngredients: nil,
		})
		if err != nil {
			log.Printf("[Nutrition Kafka Consumer] Failed to recalibrate plan on workout event: %v", err)
		} else {
			log.Printf("[Nutrition Kafka Consumer] Successfully rebalanced nutrition plan for user %s", payload.UserID)
		}
	}
}
