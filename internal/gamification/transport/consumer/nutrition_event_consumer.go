package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
)

// NutritionPayload represents the minimal inbound JSON payload for nutrition events.
type NutritionPayload struct {
	UserID       string `json:"userId"`
	ActivityDate string `json:"activityDate"` // YYYY-MM-DD
}

// NutritionEventConsumer listens for daily nutrition goal completion events.
type NutritionEventConsumer struct {
	reader           *kafka.Reader
	nutritionHandler *command.ProcessNutritionXpHandler
	streakHandler    *command.RecordStreakActivityHandler
	outboxLogRepo    port.OutboxLogRepository
}

// NewNutritionEventConsumer constructs NutritionEventConsumer.
func NewNutritionEventConsumer(
	reader *kafka.Reader,
	nutritionHandler *command.ProcessNutritionXpHandler,
	streakHandler *command.RecordStreakActivityHandler,
	outboxLogRepo port.OutboxLogRepository,
) *NutritionEventConsumer {
	return &NutritionEventConsumer{
		reader:           reader,
		nutritionHandler: nutritionHandler,
		streakHandler:    streakHandler,
		outboxLogRepo:    outboxLogRepo,
	}
}

// Start polls Kafka for nutrition events until context cancellation.
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
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] NutritionEvent consumer read error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("[Gamification] NutritionEvent consumer process error: %v", err)
			}
		}
	}
}

// ProcessMessage processes inbound nutrition goal achievement events.
//
//nolint:gocritic // kafka.Message parameter matches signature
func (c *NutritionEventConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !strings.Contains(ce.Type, "Nutrition") &&
		!strings.Contains(ce.Type, "nutrition") {
		return nil
	}

	// Idempotency check
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
		activityDate = ResolveLocalDate(time.Now())
	}
	now := time.Now().UTC()

	// 1. Award Nutrition XP (+30 XP)
	_, err := c.nutritionHandler.Handle(ctx, command.ProcessNutritionXpCommand{
		UserID:       userID,
		ActivityDate: activityDate,
		AchievedAt:   now,
	})
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

	// 2. Advance / maintain streak for rest day discipline
	_, err = c.streakHandler.Handle(ctx, command.RecordStreakActivityCommand{
		UserID:       userID,
		ActivityDate: activityDate,
		ActivityType: "NUTRITION",
		RecordedAt:   now,
	})
	if err != nil {
		log.Printf("[Gamification] Warning: streak update for nutrition event failed: %v", err)
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
