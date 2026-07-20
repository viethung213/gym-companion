package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/event"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	profileevent "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/shared/cloudevent"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	profileCompletedEventType = "contracts.supporting.profile.v1.profileCompleted"
	profileEventSource        = "services/profile-service"
	maxProcessAttempts        = 5
	initialRetryBackoff       = 250 * time.Millisecond
)

var errPermanentMessage = errors.New("permanent Kafka message error")

type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

type DeadLetterSink interface {
	Publish(
		ctx context.Context,
		message *kafka.Message,
		reason string,
		attempts int,
	) error
}

type Consumer struct {
	reader       messageReader
	inboxRepo    port.InboxRepository
	eventHandler *event.ProfileCompletedHandler
	deadLetters  DeadLetterSink
}

func NewConsumer(
	brokers []string,
	topic, groupID string,
	inboxRepo port.InboxRepository,
	eventHandler *event.ProfileCompletedHandler,
	deadLetters DeadLetterSink,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    10,
		MaxBytes:    10 * 1024 * 1024,
		StartOffset: kafka.FirstOffset,
	})

	return newConsumer(reader, inboxRepo, eventHandler, deadLetters)
}

func newConsumer(
	reader messageReader,
	inboxRepo port.InboxRepository,
	eventHandler *event.ProfileCompletedHandler,
	deadLetters DeadLetterSink,
) *Consumer {
	return &Consumer{
		reader:       reader,
		inboxRepo:    inboxRepo,
		eventHandler: eventHandler,
		deadLetters:  deadLetters,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Println("INFO: starting Coaching Kafka consumer")
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("ERROR: fetch Kafka message: %v", err)
			if !waitForRetry(ctx, initialRetryBackoff) {
				return
			}
			continue
		}

		if err := c.handleFetchedMessage(ctx, &message); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("ERROR: handle Kafka message offset %d: %v", message.Offset, err)
		}
	}
}

func (c *Consumer) handleFetchedMessage(
	ctx context.Context,
	message *kafka.Message,
) error {
	var processErr error
	attempts := 0
	for attempts < maxProcessAttempts {
		attempts++
		processErr = c.processMessage(ctx, message)
		if processErr == nil {
			return c.commit(ctx, message)
		}
		if errors.Is(processErr, errPermanentMessage) {
			break
		}

		backoff := initialRetryBackoff << (attempts - 1)
		if attempts < maxProcessAttempts && !waitForRetry(ctx, backoff) {
			return ctx.Err()
		}
	}

	reason := "retry_exhausted"
	if errors.Is(processErr, errPermanentMessage) {
		reason = "invalid_message"
	}
	for {
		err := c.deadLetters.Publish(ctx, message, reason, attempts)
		if err == nil {
			return c.commit(ctx, message)
		}
		log.Printf("ERROR: publish message offset %d to DLQ: %v", message.Offset, err)
		if !waitForRetry(ctx, initialRetryBackoff) {
			return ctx.Err()
		}
	}
}

func (c *Consumer) processMessage(
	ctx context.Context,
	message *kafka.Message,
) error {
	envelope, profileCompleted, err := decodeProfileCompleted(message)
	if err != nil {
		return err
	}

	processed, err := c.inboxRepo.IsProcessed(ctx, envelope.ID)
	if err != nil {
		return fmt.Errorf("check inbox processed: %w", err)
	}
	if processed {
		return nil
	}

	if err := c.eventHandler.Handle(ctx, profileCompleted); err != nil {
		return fmt.Errorf("handle ProfileCompleted event: %w", err)
	}

	if err := c.inboxRepo.MarkProcessed(
		ctx,
		envelope.ID,
		envelope.Type,
		message.Value,
		string(message.Key),
	); err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}

	return nil
}

func decodeProfileCompleted(
	message *kafka.Message,
) (*cloudevent.Envelope, *event.ProfileCompletedEvent, error) {
	envelope, err := cloudevent.Decode(message.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errPermanentMessage, err)
	}
	if envelope.Type != profileCompletedEventType {
		return nil, nil, fmt.Errorf("%w: unsupported event type", errPermanentMessage)
	}
	if envelope.Source != profileEventSource {
		return nil, nil, fmt.Errorf("%w: unsupported event source", errPermanentMessage)
	}

	var payload profileevent.ProfileCompleted
	if err := protojson.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, nil, fmt.Errorf(
			"%w: decode ProfileCompleted data: %w",
			errPermanentMessage,
			err,
		)
	}
	if strings.TrimSpace(payload.UserId) == "" {
		return nil, nil, fmt.Errorf(
			"%w: ProfileCompleted userId is required",
			errPermanentMessage,
		)
	}
	experienceLevel := strings.ToLower(strings.TrimSpace(payload.ExperienceLevel))
	switch experienceLevel {
	case "beginner", "intermediate", "advanced":
	default:
		return nil, nil, fmt.Errorf(
			"%w: unsupported experienceLevel",
			errPermanentMessage,
		)
	}

	return envelope, &event.ProfileCompletedEvent{
		UserID:             payload.UserId,
		Goals:              payload.Goals,
		RegisteredInjuries: payload.RegisteredInjuries,
		ExperienceLevel:    experienceLevel,
	}, nil
}

func (c *Consumer) commit(ctx context.Context, message *kafka.Message) error {
	for {
		err := c.reader.CommitMessages(ctx, *message)
		if err == nil {
			return nil
		}
		log.Printf("ERROR: commit Kafka message offset %d: %v", message.Offset, err)

		if !waitForRetry(ctx, initialRetryBackoff) {
			return fmt.Errorf("commit Kafka message: %w", ctx.Err())
		}
	}
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close Kafka reader: %w", err)
	}

	return nil
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
