package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
)

// Publisher implements port.BrokerPublisher to deliver outbox records to Kafka broker.
type Publisher struct {
	writer *kafka.Writer
}

// Compile-time interface check.
var _ port.BrokerPublisher = (*Publisher)(nil)

// NewPublisher constructs a new Publisher.
func NewPublisher(writer *kafka.Writer) *Publisher {
	return &Publisher{writer: writer}
}

// PublishBatch writes a batch of outbox records to Kafka topic.
func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 || p.writer == nil {
		return nil
	}

	msgs := make([]kafka.Message, len(records))
	for i, r := range records {
		msgs[i] = kafka.Message{
			Key:   []byte(r.PartitionKey),
			Value: r.Payload,
		}
	}

	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("write gamification kafka batch messages: %w", err)
	}

	return nil
}
