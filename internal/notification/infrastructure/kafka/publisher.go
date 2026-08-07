package kafka

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

var _ port.BrokerPublisher = (*Publisher)(nil)

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(writer *kafka.Writer) *Publisher {
	return &Publisher{writer: writer}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if p.writer == nil || len(records) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, 0, len(records))
	for _, rec := range records {
		msgs = append(msgs, kafka.Message{
			Key:   []byte(rec.PartitionKey),
			Value: rec.Payload,
		})
	}

	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("write kafka messages: %w", err)
	}

	log.Printf("[Notification Kafka Publisher] Published %d messages to Kafka", len(msgs))
	return nil
}

func (p *Publisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
