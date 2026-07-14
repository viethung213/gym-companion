package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
)

type Publisher struct {
	writer *kafka.Writer
}

var _ port.BrokerPublisher = (*Publisher)(nil)

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,             // Enables topic auto-creation
			RequiredAcks:           kafka.RequireAll, // Robust delivery
		},
	}
}

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
		return fmt.Errorf("write exercise kafka batch messages: %w", err)
	}
	return nil
}

func (p *Publisher) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close exercise kafka writer: %w", err)
	}
	return nil
}
