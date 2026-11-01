package consumer

import (
	"context"
	"encoding/json"
	"fmt"
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
	runConsumerLoop(ctx, "NutritionEvent", c.reader, c.ProcessMessage)
}

func (c *NutritionEventConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	return processIdempotentCloudEvent(
		ctx,
		msg,
		c.outboxLogRepo,
		func(eventType string) bool {
			return strings.Contains(eventType, "Nutrition") || strings.Contains(eventType, "nutrition")
		},
		func(ctx context.Context, ce *CloudEvent, msg *kafka.Message) error {
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

			if c.handler == nil {
				return nil
			}
			return c.handler.Handle(ctx, command.ProcessNutritionActivityCommand{
				UserID:       userID,
				ActivityDate: activityDate,
				RecordedAt:   time.Now().UTC(),
			})
		},
	)
}
