package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

func runConsumerLoop(
	ctx context.Context,
	name string,
	reader *kafka.Reader,
	dlqWriter sharedKafka.MessageWriter,
	processFn func(ctx context.Context, msg *kafka.Message) error,
) {
	sharedKafka.RunConsumerLoop(
		ctx,
		reader,
		sharedKafka.ConsumerConfig{
			Name:        name,
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   dlqWriter,
		},
		processFn,
	)
}

func processIdempotentCloudEvent(
	ctx context.Context,
	msg *kafka.Message,
	outboxLogRepo port.OutboxLogRepository,
	matchType func(eventType string) bool,
	handleData func(ctx context.Context, ce *CloudEvent, msg *kafka.Message) error,
) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}

	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if !matchType(ce.Type) {
		return nil
	}

	if outboxLogRepo != nil && ce.ID != "" {
		processed, err := outboxLogRepo.IsProcessed(ctx, ce.ID)
		if err == nil && processed {
			return nil
		}
	}

	if err := handleData(ctx, &ce, msg); err != nil {
		if outboxLogRepo != nil && ce.ID != "" {
			_ = outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
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

	if outboxLogRepo != nil && ce.ID != "" {
		_ = outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      ce.ID,
			EventType:    ce.Type,
			Payload:      msg.Value,
			PartitionKey: string(msg.Key),
			Status:       "PROCESSED",
		})
	}

	return nil
}
