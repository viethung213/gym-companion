package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
)

type Publisher struct {
	writer *kafka.Writer
}

var _ port.BrokerPublisher = (*Publisher)(nil)

func NewPublisher(writer *kafka.Writer) *Publisher {
	return &Publisher{writer: writer}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, len(records))
	for i, r := range records {
		msgs[i] = kafka.Message{
			Key:   []byte(r.PartitionKey),
			Value: r.Payload,
		}
	}

	err := p.writer.WriteMessages(ctx, msgs...)
	if err != nil {
		return fmt.Errorf("write kafka profile batch messages: %w", err)
	}
	return nil
}

func (p *Publisher) Close() error {
	return nil
}
