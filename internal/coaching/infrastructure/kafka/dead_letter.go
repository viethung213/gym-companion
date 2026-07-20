package kafka

import (
	"context"
	"fmt"
	"strconv"

	"github.com/segmentio/kafka-go"
)

type DeadLetterPublisher struct {
	writer *kafka.Writer
	topic  string
}

var _ DeadLetterSink = (*DeadLetterPublisher)(nil)

func NewDeadLetterPublisher(
	writer *kafka.Writer,
	topic string,
) *DeadLetterPublisher {
	return &DeadLetterPublisher{
		writer: writer,
		topic:  topic,
	}
}

func (p *DeadLetterPublisher) Publish(
	ctx context.Context,
	message *kafka.Message,
	reason string,
	attempts int,
) error {
	headers := append([]kafka.Header(nil), message.Headers...)
	headers = append(headers,
		kafka.Header{Key: "dlqreason", Value: []byte(reason)},
		kafka.Header{Key: "originaltopic", Value: []byte(message.Topic)},
		kafka.Header{
			Key:   "originalpartition",
			Value: []byte(strconv.Itoa(message.Partition)),
		},
		kafka.Header{
			Key:   "originaloffset",
			Value: []byte(strconv.FormatInt(message.Offset, 10)),
		},
		kafka.Header{Key: "attempts", Value: []byte(strconv.Itoa(attempts))},
	)

	dlqMessage := kafka.Message{
		Topic:   p.topic,
		Key:     message.Key,
		Value:   message.Value,
		Headers: headers,
	}
	if err := p.writer.WriteMessages(ctx, dlqMessage); err != nil {
		return fmt.Errorf("write dead-letter message: %w", err)
	}

	return nil
}
