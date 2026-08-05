package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
)

type WorkoutSessionCompletedPayload struct {
	SessionID           string    `json:"sessionId"`
	UserID              string    `json:"userId"`
	TotalCaloriesBurned float64   `json:"totalCaloriesBurned"`
	CompletedAt         time.Time `json:"completedAt"`
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

	log.Println("[Nutrition Kafka Consumer] Started listening to Kafka topic 'workout.session.completed'")

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

func (c *Consumer) handleMessage(ctx context.Context, msg segmentio.Message) {
	var payload WorkoutSessionCompletedPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		log.Printf("[Nutrition Kafka Consumer] Error unmarshaling event payload: %v", err)
		return
	}

	log.Printf("[Nutrition Kafka Consumer] Received WorkoutSessionCompleted Event: UserID=%s, Burned=%.2f kcal",
		payload.UserID, payload.TotalCaloriesBurned)

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
