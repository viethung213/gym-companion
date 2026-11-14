package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
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
		slog.InfoContext(ctx, "Kafka reader is nil, skipping consumer loop")
		return nil
	}

	slog.InfoContext(ctx, "Started listening to Kafka topic", slog.String("topic", "workout_execution.events"))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Shutting down Kafka consumer")
			return nil
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.ErrorContext(ctx, "Error fetching message", slog.Any("error", err))
				continue
			}

			c.handleMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				slog.ErrorContext(ctx, "Error committing message", slog.Any("error", err))
			}
		}
	}
}

//nolint:gocritic // msg is passed by value per kafka library signature
func (c *Consumer) handleMessage(ctx context.Context, msg segmentio.Message) {
	var env cloudEventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		slog.ErrorContext(ctx, "Error unmarshaling CloudEvent envelope", slog.Any("error", err))
		return
	}

	if !strings.HasSuffix(env.Type, "workoutSessionCompleted") && !strings.HasSuffix(env.Type, "WorkoutSessionCompleted") {
		return
	}

	// 1. Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			slog.WarnContext(ctx, "Warning checking outbox log idempotency", slog.Any("error", err))
		} else if processed {
			slog.DebugContext(ctx, "Event already processed in outbox_log, skipping", slog.String("event_id", env.ID))
			return
		}
	}

	var payload WorkoutSessionCompletedPayload
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			slog.ErrorContext(ctx, "Error unmarshaling event payload", slog.Any("error", err))
			c.saveLog(ctx, msg.Value, env, "", err)
			return
		}
	} else {
		// Fallback if payload is not wrapped in data
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			slog.ErrorContext(ctx, "Error unmarshaling raw payload", slog.Any("error", err))
			c.saveLog(ctx, msg.Value, env, "", err)
			return
		}
	}

	slog.InfoContext(ctx, "Received WorkoutSessionCompleted Event",
		slog.String("user_id", payload.UserID),
		slog.String("session_id", payload.SessionID),
		slog.Float64("volume", payload.TotalVolume))

	var processErr error
	if c.recalibrateHandler != nil && payload.UserID != "" {
		_, processErr = c.recalibrateHandler.Handle(ctx, command.RecalibratePlanWithPantryCommand{
			UserID:               payload.UserID,
			PlanDate:             time.Now(),
			AvailableIngredients: nil,
		})
		if processErr != nil {
			slog.ErrorContext(ctx, "Failed to recalibrate plan on workout event", slog.Any("error", processErr))
		} else {
			slog.InfoContext(ctx, "Successfully rebalanced nutrition plan for user", slog.String("user_id", payload.UserID))
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
		slog.WarnContext(ctx, "Warning: failed to save outbox log for event", slog.String("event_id", env.ID), slog.Any("error", saveErr))
	}
}
