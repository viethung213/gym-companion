package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type Publisher struct {
	writer *kafka.Writer
	topic  string
}

var _ port.BrokerPublisher = (*Publisher)(nil)

func NewPublisher(writer *kafka.Writer, topic string) *Publisher {
	return &Publisher{writer: writer, topic: topic}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, len(records))
	for i, r := range records {
		msgs[i] = kafka.Message{
			Topic: p.topic,
			Key:   []byte(r.PartitionKey),
			Value: r.Payload,
			Headers: []kafka.Header{
				{Key: "event_id", Value: []byte(r.EventID)},
				{Key: "event_type", Value: []byte(r.EventType)},
			},
		}
	}

	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("write kafka batch messages: %w", err)
	}
	return nil
}

func (p *Publisher) Close() error {
	return nil
}
