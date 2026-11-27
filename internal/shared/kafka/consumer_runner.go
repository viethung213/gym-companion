package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

// MessageReader abstracts kafka.Reader methods for testing and decoupling.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

// SystemDLQTopic defines the system-wide Dead Letter Queue topic used across all modules (Option A).
const SystemDLQTopic = "system.events.dlq"

// MessageWriter abstracts kafka.Writer methods for testing and decoupling.
type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

// ConsumerConfig configures the execution of RunConsumerLoop.
type ConsumerConfig struct {
	Name        string                                                             // Consumer name for telemetry/logging
	MaxRetries  int                                                                // Maximum in-memory attempts (default 3)
	BaseBackoff time.Duration                                                      // Base backoff for incremental retry (default 100ms)
	DLQWriter   MessageWriter                                                      // Optional writer to publish exhausted messages to DLQ topic
	OnDLQ       func(ctx context.Context, msg *kafka.Message, lastErr error) error // Optional callback when all retries fail (e.g. outbox_log status DEAD_LETTER)
}

func isNilReader(reader MessageReader) bool {
	if reader == nil {
		return true
	}
	val := reflect.ValueOf(reader)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return true
	}
	return false
}

// RunConsumerLoop coordinates safe message consumption:
// FetchMessage -> In-memory Retry up to MaxRetries with backoff -> DLQ if exhausted -> CommitMessages.
func RunConsumerLoop(
	ctx context.Context,
	reader MessageReader,
	cfg ConsumerConfig,
	processFn func(ctx context.Context, msg *kafka.Message) error,
) {
	if isNilReader(reader) {
		slog.InfoContext(ctx, "Kafka consumer runner skipping: reader is nil", slog.String("consumer", cfg.Name))
		return
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 100 * time.Millisecond
	}

	slog.InfoContext(ctx, "Kafka consumer runner started",
		slog.String("consumer", cfg.Name),
		slog.Int("max_retries", cfg.MaxRetries),
		slog.Duration("base_backoff", cfg.BaseBackoff),
	)

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Kafka consumer runner stopped: context cancelled", slog.String("consumer", cfg.Name))
			return
		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.ErrorContext(ctx, "Kafka consumer fetch message error",
					slog.String("consumer", cfg.Name),
					slog.Any("error", err),
				)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var lastErr error
			for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
				func() {
					defer func() {
						if r := recover(); r != nil {
							lastErr = fmt.Errorf("panic in message handler: %v", r)
							slog.ErrorContext(ctx, "PANIC RECOVERED in Kafka message processing",
								slog.String("consumer", cfg.Name),
								slog.String("topic", msg.Topic),
								slog.Int64("offset", msg.Offset),
								slog.Int("attempt", attempt),
								slog.Any("panic", r),
							)
						}
					}()
					lastErr = processFn(ctx, &msg)
				}()
				if lastErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}

				backoff := time.Duration(attempt) * cfg.BaseBackoff
				slog.WarnContext(ctx, "Kafka consumer message processing failed, retrying",
					slog.String("consumer", cfg.Name),
					slog.Int("attempt", attempt),
					slog.Int("max_retries", cfg.MaxRetries),
					slog.Duration("backoff", backoff),
					slog.Any("error", lastErr),
				)
				time.Sleep(backoff)
			}

			if lastErr != nil {
				slog.ErrorContext(ctx, "Kafka consumer exhausted all retries",
					slog.String("consumer", cfg.Name),
					slog.String("topic", msg.Topic),
					slog.Int64("offset", msg.Offset),
					slog.Int("attempts", cfg.MaxRetries),
					slog.Any("error", lastErr),
				)

				dlqHandled := false

				if cfg.OnDLQ != nil {
					if cbErr := cfg.OnDLQ(ctx, &msg, lastErr); cbErr != nil {
						slog.ErrorContext(ctx, "Kafka consumer OnDLQ callback error",
							slog.String("consumer", cfg.Name),
							slog.Any("error", cbErr),
						)
					} else {
						dlqHandled = true
					}
				}

				if cfg.DLQWriter != nil {
					dlqHeaders := make([]kafka.Header, len(msg.Headers), len(msg.Headers)+4)
					copy(dlqHeaders, msg.Headers)
					dlqHeaders = append(dlqHeaders,
						kafka.Header{Key: "x-original-topic", Value: []byte(msg.Topic)},
						kafka.Header{Key: "x-retry-count", Value: []byte(strconv.Itoa(cfg.MaxRetries))},
						kafka.Header{Key: "x-error-message", Value: []byte(lastErr.Error())},
						kafka.Header{Key: "x-failed-at", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
					)
					dlqMsg := kafka.Message{
						Key:     msg.Key,
						Value:   msg.Value,
						Headers: dlqHeaders,
					}
					if pubErr := cfg.DLQWriter.WriteMessages(ctx, dlqMsg); pubErr != nil {
						slog.ErrorContext(ctx, "Kafka consumer failed to publish to DLQ topic, aborting offset commit to prevent silent message loss",
							slog.String("consumer", cfg.Name),
							slog.Any("error", pubErr),
						)
						dlqHandled = false
					} else {
						dlqHandled = true
					}
				}

				if !dlqHandled {
					slog.WarnContext(ctx, "Message processing failed without successful DLQ routing; aborting offset commit to preserve At-Least-Once delivery",
						slog.String("consumer", cfg.Name),
						slog.String("topic", msg.Topic),
						slog.Int64("offset", msg.Offset),
					)
					time.Sleep(cfg.BaseBackoff)
					continue
				}
			}

			if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
				slog.ErrorContext(ctx, "Kafka consumer commit message error",
					slog.String("consumer", cfg.Name),
					slog.String("topic", msg.Topic),
					slog.Int64("offset", msg.Offset),
					slog.Any("error", commitErr),
				)
			}
		}
	}
}
