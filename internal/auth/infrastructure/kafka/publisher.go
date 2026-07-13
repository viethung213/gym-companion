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

// NewPublisher creates a new Publisher configured with the provided brokers.
func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,             // Enables topic auto-creation if not exists
			RequiredAcks:           kafka.RequireAll, // Ensure robust delivery (ACID-like safety)
		},
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

// Close releases the writer connection pool resource.
func (p *Publisher) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}
