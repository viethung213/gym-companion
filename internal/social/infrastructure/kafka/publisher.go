package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(writer *kafka.Writer, notifWriters ...*kafka.Writer) *Publisher {
	w := writer
	if len(notifWriters) > 0 && notifWriters[0] != nil {
		w = notifWriters[0]
	}
	return &Publisher{
		writer: w,
	}
}

func (p *Publisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 || p.writer == nil {
		return nil
	}

	var notifMsgs []kafka.Message
	for _, r := range records {
		if isNotificationEvent(r.EventType) {
			notifMsgs = append(notifMsgs, kafka.Message{
				Key:   []byte(r.PartitionKey),
				Value: r.Payload,
			})
		}
	}

	if len(notifMsgs) > 0 {
		if err := p.writer.WriteMessages(ctx, notifMsgs...); err != nil {
			return fmt.Errorf("write kafka notification batch messages: %w", err)
		}
	}

	return nil
}

func isNotificationEvent(eventType string) bool {
	return strings.Contains(eventType, "NormalPriorityNotificationRequested") ||
		strings.Contains(eventType, "HighPriorityNotificationRequested")
}
