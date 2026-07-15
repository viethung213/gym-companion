package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// Publisher handles writing messages directly to Kafka topics.
type Publisher struct {
	writer *kafka.Writer
}

// NewPublisher creates a new Publisher with the provided shared kafka.Writer.
func NewPublisher(writer *kafka.Writer) *Publisher {
	return &Publisher{
		writer: writer,
	}
}

// PublishBatch writes multiple outbox records to Kafka in a single batch.
func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, len(records))
	for i, r := range records {
		msgs[i] = kafka.Message{
			Topic: r.EventType,
			Key:   []byte(r.PartitionKey),
			Value: r.Payload,
		}
	}

	err := p.writer.WriteMessages(ctx, msgs...)
	if err != nil {
		return fmt.Errorf("write kafka batch messages: %w", err)
	}
	return nil
}

// Close is a no-op as the shared Kafka connection Registry handles lifecycle management.
func (p *Publisher) Close() error {
	return nil
}
