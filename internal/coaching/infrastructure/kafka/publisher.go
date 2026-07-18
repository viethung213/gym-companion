package kafka

import (
	"context"
	"fmt"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type Publisher struct {
	writer *segmentio.Writer
}

func NewPublisher(writer *segmentio.Writer) *Publisher {
	return &Publisher{writer: writer}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	messages := make([]segmentio.Message, 0, len(records))
	for _, record := range records {
		messages = append(messages, segmentio.Message{
			Topic: record.EventType,
			Key:   []byte(record.PartitionKey),
			Value: record.Payload,
		})
	}
	if len(messages) == 0 {
		return nil
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish coaching events: %w", err)
	}
	return nil
}

var _ port.EventPublisher = (*Publisher)(nil)
