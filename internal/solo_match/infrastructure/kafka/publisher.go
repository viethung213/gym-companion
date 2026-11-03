package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

// Publisher implements port.BrokerPublisher for Kafka.
type Publisher struct {
	writer *kafka.Writer
}

var _ port.BrokerPublisher = (*Publisher)(nil)

// NewPublisher creates a new Kafka publisher instance.
func NewPublisher(writer *kafka.Writer) *Publisher {
	return &Publisher{
		writer: writer,
	}
}

// PublishBatch writes multiple outbox records to Kafka in a single batch.
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
		return fmt.Errorf("write solo_match kafka batch messages: %w", err)
	}

	return nil
}

// Close is a no-op as the shared Kafka connection registry handles writer lifecycle.
func (p *Publisher) Close() error {
	return nil
}
