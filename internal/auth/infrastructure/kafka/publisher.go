package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// Publisher handles writing messages directly to Kafka topics.
type Publisher struct {
	defaultWriter      *kafka.Writer
	highPriorityWriter *kafka.Writer
}

// NewPublisher creates a new Publisher with the provided shared kafka.Writer and optional high-priority writer.
func NewPublisher(writer *kafka.Writer, highPriorityWriter ...*kafka.Writer) *Publisher {
	pub := &Publisher{
		defaultWriter: writer,
	}
	if len(highPriorityWriter) > 0 {
		pub.highPriorityWriter = highPriorityWriter[0]
	}
	return pub
}

// PublishBatch writes multiple outbox records to Kafka in a single batch,
// routing high-priority notification events to the dedicated high-priority writer if available.
func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	var defaultMsgs []kafka.Message
	var highPriorityMsgs []kafka.Message

	for _, r := range records {
		msg := kafka.Message{
			Key:   []byte(r.PartitionKey),
			Value: r.Payload,
		}

		if r.EventType == "contracts.generic.notification.v1.event.HighPriorityNotificationRequested" {
			if p.highPriorityWriter == nil {
				return fmt.Errorf("high-priority kafka writer is not configured for event %s (%s)", r.EventID, r.EventType)
			}
			highPriorityMsgs = append(highPriorityMsgs, msg)
		} else {
			if p.defaultWriter == nil {
				return fmt.Errorf("default kafka writer is not configured for event %s (%s)", r.EventID, r.EventType)
			}
			defaultMsgs = append(defaultMsgs, msg)
		}
	}

	if len(defaultMsgs) > 0 {
		if err := p.defaultWriter.WriteMessages(ctx, defaultMsgs...); err != nil {
			return fmt.Errorf("write default kafka messages: %w", err)
		}
	}

	if len(highPriorityMsgs) > 0 {
		if err := p.highPriorityWriter.WriteMessages(ctx, highPriorityMsgs...); err != nil {
			return fmt.Errorf("write high priority kafka messages: %w", err)
		}
	}

	return nil
}

// Close is a no-op as the shared Kafka connection Registry handles lifecycle management.
func (p *Publisher) Close() error {
	return nil
}
