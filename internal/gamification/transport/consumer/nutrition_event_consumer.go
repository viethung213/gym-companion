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
	txManager        port.TransactionManager
	outboxLogRepo    port.OutboxLogRepository
}

// NewNutritionEventConsumer constructs NutritionEventConsumer.
func NewNutritionEventConsumer(
	reader *kafka.Reader,
	nutritionHandler *command.ProcessNutritionXpHandler,
	streakHandler *command.RecordStreakActivityHandler,
	txManager port.TransactionManager,
	outboxLogRepo port.OutboxLogRepository,
) *NutritionEventConsumer {
	return &NutritionEventConsumer{
		reader:           reader,
		nutritionHandler: nutritionHandler,
		streakHandler:    streakHandler,
		txManager:        txManager,
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
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] NutritionEvent consumer commit error: %v", err)
			}
		}
	}
}

// ProcessMessage processes inbound nutrition goal achievement events atomically.
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

	var processErr error
	if c.txManager != nil {
		// Atomic transaction adhering strictly to user_streak -> user_xp lock ordering to prevent deadlocks
		processErr = c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			// 1. Advance / maintain streak for rest day discipline
			if c.streakHandler != nil {
				_, sErr := c.streakHandler.Handle(txCtx, command.RecordStreakActivityCommand{
					UserID:       userID,
					ActivityDate: activityDate,
					ActivityType: "NUTRITION",
					RecordedAt:   now,
				})
				if sErr != nil {
					return fmt.Errorf("record nutrition streak activity: %w", sErr)
				}
			}

			// 2. Award Nutrition XP (+30 XP)
			if c.nutritionHandler != nil {
				_, nErr := c.nutritionHandler.Handle(txCtx, command.ProcessNutritionXpCommand{
					UserID:       userID,
					ActivityDate: activityDate,
					AchievedAt:   now,
				})
				if nErr != nil {
					return fmt.Errorf("process nutrition xp: %w", nErr)
				}
			}

			return nil
		})
	} else {
		// Fallback for environments without transaction manager
		if c.streakHandler != nil {
			_, sErr := c.streakHandler.Handle(ctx, command.RecordStreakActivityCommand{
				UserID:       userID,
				ActivityDate: activityDate,
				ActivityType: "NUTRITION",
				RecordedAt:   now,
			})
			if sErr != nil {
				processErr = fmt.Errorf("record nutrition streak activity: %w", sErr)
			}
		}

		if processErr == nil && c.nutritionHandler != nil {
			_, nErr := c.nutritionHandler.Handle(ctx, command.ProcessNutritionXpCommand{
				UserID:       userID,
				ActivityDate: activityDate,
				AchievedAt:   now,
			})
			if nErr != nil {
				processErr = fmt.Errorf("process nutrition xp: %w", nErr)
			}
		}
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
