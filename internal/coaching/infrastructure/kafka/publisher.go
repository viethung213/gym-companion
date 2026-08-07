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

var _ port.BrokerPublisher = (*Publisher)(nil)

func NewPublisher(writer *segmentio.Writer) *Publisher {
	return &Publisher{writer: writer}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	messages := make([]segmentio.Message, 0, len(records))
	for _, rec := range records {
		if rec == nil {
			continue
		}
		messages = append(messages, segmentio.Message{
			Key:   []byte(rec.PartitionKey),
			Value: rec.Payload,
			Headers: []segmentio.Header{
				{Key: "eventType", Value: []byte(rec.EventType)},
				{Key: "eventId", Value: []byte(rec.EventID)},
			},
		})
	}

	if p.writer != nil {
		if err := p.writer.WriteMessages(ctx, messages...); err != nil {
			return fmt.Errorf("kafka publisher write messages: %w", err)
		}
	}

	return nil
}

func (p *Publisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
