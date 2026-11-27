package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type WorkoutSessionCompletedPayload struct {
	SessionID           string  `json:"sessionId"`
	UserID              string  `json:"userId"`
	TotalCaloriesBurned float64 `json:"totalCaloriesBurned"`
	TotalVolume         float64 `json:"totalVolume"`
	CompletedAt         string  `json:"completedAt"`
}

// WorkoutEventConsumer listens to workout execution events and adjusts nutrition plan macros.
type WorkoutEventConsumer struct {
	reader                *segmentio.Reader
	dlqWriter             sharedKafka.MessageWriter
	workoutSurplusHandler *command.RebalanceWorkoutNutritionHandler
	outboxLogRepo         port.OutboxLogRepository
}

func (c *WorkoutEventConsumer) WithDLQWriter(writer sharedKafka.MessageWriter) *WorkoutEventConsumer {
	c.dlqWriter = writer
	return c
}

func NewWorkoutEventConsumer(
	reader *segmentio.Reader,
	workoutSurplusHandler *command.RebalanceWorkoutNutritionHandler,
	outboxLogRepo port.OutboxLogRepository,
) *WorkoutEventConsumer {
	return &WorkoutEventConsumer{
		reader:                reader,
		workoutSurplusHandler: workoutSurplusHandler,
		outboxLogRepo:         outboxLogRepo,
	}
}

func (c *WorkoutEventConsumer) Start(ctx context.Context) error {
	if c.reader == nil {
		slog.InfoContext(ctx, "Kafka reader is nil, skipping consumer loop")
		return nil
	}

	slog.InfoContext(ctx, "Started listening to Kafka topic", slog.String("topic", "workout_execution.events"))
	sharedKafka.RunConsumerLoop(
		ctx,
		c.reader,
		sharedKafka.ConsumerConfig{
			Name:        "NutritionWorkoutExecutionConsumer",
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   c.dlqWriter,
		},
		func(ctx context.Context, msg *segmentio.Message) error {
			return c.HandleMessage(ctx, *msg)
		},
	)
	return nil
}

//nolint:gocritic // msg is passed by value per kafka library signature
func (c *WorkoutEventConsumer) HandleMessage(ctx context.Context, msg segmentio.Message) error {
	var env CloudEventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		slog.ErrorContext(ctx, "Error unmarshaling CloudEvent envelope", slog.Any("error", err))
		return err
	}

	if !strings.HasSuffix(env.Type, "workoutSessionCompleted") && !strings.HasSuffix(env.Type, "WorkoutSessionCompleted") {
		return nil
	}

	// 1. Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			slog.WarnContext(ctx, "Warning checking outbox log idempotency", slog.Any("error", err))
		} else if processed {
			slog.DebugContext(ctx, "Event already processed in outbox_log, skipping", slog.String("event_id", env.ID))
			return nil
		}
	}

	var payload WorkoutSessionCompletedPayload
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			slog.ErrorContext(ctx, "Error unmarshaling event payload", slog.Any("error", err))
			c.saveLog(ctx, msg.Value, env, "", err)
			return err
		}
	} else {
		// Fallback if payload is not wrapped in data
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			slog.ErrorContext(ctx, "Error unmarshaling raw payload", slog.Any("error", err))
			c.saveLog(ctx, msg.Value, env, "", err)
			return err
		}
	}

	slog.InfoContext(ctx, "Received WorkoutSessionCompleted Event",
		slog.String("user_id", payload.UserID),
		slog.String("session_id", payload.SessionID),
		slog.Float64("volume", payload.TotalVolume))

	var processErr error
	if c.workoutSurplusHandler != nil && payload.UserID != "" {
		_, processErr = c.workoutSurplusHandler.Handle(ctx, &command.RebalanceWorkoutNutritionCommand{
			UserID:      payload.UserID,
			PlanDate:    time.Now(),
			SessionID:   payload.SessionID,
			TotalVolume: payload.TotalVolume,
		})
		if processErr != nil {
			slog.ErrorContext(ctx, "Failed to rebalance nutrition plan on workout event", slog.Any("error", processErr))
		} else {
			slog.InfoContext(ctx, "Successfully rebalanced nutrition plan with workout surplus",
				slog.String("user_id", payload.UserID),
				slog.String("session_id", payload.SessionID))
		}
	}

	c.saveLog(ctx, msg.Value, env, payload.UserID, processErr)
	return processErr
}

//nolint:gocritic // env envelope value object is passed by value per helper design
func (c *WorkoutEventConsumer) saveLog(ctx context.Context, rawPayload []byte, env CloudEventEnvelope, userID string, err error) {
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
