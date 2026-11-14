package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// MessageHandler defines the contract for processing raw event payloads.
type MessageHandler interface {
	HandleMessage(ctx context.Context, rawPayload []byte) error
}

// KafkaMessageReader abstracts kafka.Reader for testing and modularity.
type KafkaMessageReader interface {
	ReadMessage(ctx context.Context) (kafka.Message, error)
	Close() error
}

// KafkaConsumerWorker handles the background polling loop for a specific Kafka topic reader.
type KafkaConsumerWorker struct {
	name     string
	reader   KafkaMessageReader
	handlers []MessageHandler
}

// NewKafkaConsumerWorker constructs a KafkaConsumerWorker with a given reader and dispatch handlers.
func NewKafkaConsumerWorker(name string, reader KafkaMessageReader, handlers ...MessageHandler) *KafkaConsumerWorker {
	return &KafkaConsumerWorker{
		name:     name,
		reader:   reader,
		handlers: handlers,
	}
}

// Start begins the message polling loop until the provided context is cancelled.
func (w *KafkaConsumerWorker) Start(ctx context.Context) {
	if w.reader == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "PANIC RECOVERED in SoloMatch consumer", slog.String("worker", w.name), slog.Any("panic", r))
		}
		if closeErr := w.reader.Close(); closeErr != nil {
			slog.WarnContext(ctx, "Error closing SoloMatch reader", slog.String("worker", w.name), slog.String("error", closeErr.Error()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := w.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					return
				}
				slog.ErrorContext(ctx, "Kafka reader error in SoloMatch consumer", slog.String("worker", w.name), slog.String("error", err.Error()))
				time.Sleep(1 * time.Second)
				continue
			}

			w.dispatchMessage(ctx, msg.Value)
		}
	}
}

func (w *KafkaConsumerWorker) dispatchMessage(ctx context.Context, rawPayload []byte) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "PANIC in processing event in SoloMatch consumer", slog.String("worker", w.name), slog.Any("panic", r))
		}
	}()

	for _, h := range w.handlers {
		if h == nil {
			continue
		}
		if err := h.HandleMessage(ctx, rawPayload); err != nil {
			slog.ErrorContext(ctx, "SoloMatch consumer handler error", slog.String("worker", w.name), slog.String("error", err.Error()))
		}
	}
}
