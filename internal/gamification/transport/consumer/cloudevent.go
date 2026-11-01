package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
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
	processFn func(ctx context.Context, msg *kafka.Message) error,
) {
	if reader == nil {
		return
	}
	log.Printf("[Gamification] Starting %s Kafka Consumer...", name)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Gamification] Stopping %s Kafka Consumer due to context cancellation.", name)
			return
		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] %s consumer fetch error: %v", name, err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = processFn(ctx, &msg)
				if processErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
			}

			if processErr != nil {
				log.Printf("[Gamification] %s consumer skipping failed message after 3 retries (recorded in outbox_log): %v", name, processErr)
			}

			if err := reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] %s consumer commit error: %v", name, err)
			}
		}
	}
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
