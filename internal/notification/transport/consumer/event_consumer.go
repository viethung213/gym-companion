package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type cloudEventEnvelope struct {
	ID     string          `json:"id"`
	Source string          `json:"source"`
	Type   string          `json:"type"`
	Time   string          `json:"time"`
	Data   json.RawMessage `json:"data"`
}

// NotificationEventConsumer coordinates Kafka message consumption and delegates
// internal vs external events to specialized handlers.
type NotificationEventConsumer struct {
	reader          sharedKafka.MessageReader
	dlqWriter       sharedKafka.MessageWriter
	selfHandler     *SelfEventHandler
	externalHandler *ExternalEventHandler
}

func NewNotificationEventConsumer(
	reader sharedKafka.MessageReader,
	sendPushHandler *command.SendPushNotificationHandler,
	sendEmailHandler *command.SendEmailNotificationHandler,
	sendSMSHandler *command.SendSMSNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *NotificationEventConsumer {
	selfHandler := NewSelfEventHandler(sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo)
	externalHandler := NewExternalEventHandler(sendPushHandler, outboxLogRepo)

	return &NotificationEventConsumer{
		reader:          reader,
		selfHandler:     selfHandler,
		externalHandler: externalHandler,
	}
}

// WithDLQWriter attaches an optional dead letter queue writer to route unrecoverable failures.
func (c *NotificationEventConsumer) WithDLQWriter(writer sharedKafka.MessageWriter) *NotificationEventConsumer {
	c.dlqWriter = writer
	return c
}

// WithDeviceRepo attaches an optional device repository to resolve all active users on broadcast.
func (c *NotificationEventConsumer) WithDeviceRepo(repo repository.DeviceRepository) *NotificationEventConsumer {
	if c.selfHandler != nil {
		c.selfHandler.WithDeviceRepo(repo)
	}
	return c
}

// Start begins consuming Kafka messages continuously until the context is canceled.
func (c *NotificationEventConsumer) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "Panic recovered in NotificationEventConsumer.Start", slog.Any("panic", r))
		}
	}()

	if c.reader == nil {
		slog.InfoContext(ctx, "Notification consumer skipping: Kafka reader is nil")
		return
	}

	slog.InfoContext(ctx, "Notification consumer started, listening for events")
	sharedKafka.RunConsumerLoop(
		ctx,
		c.reader,
		sharedKafka.ConsumerConfig{
			Name:        "NotificationEventConsumer",
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   c.dlqWriter,
		},
		func(ctx context.Context, msg *kafka.Message) error {
			return c.ProcessMessage(ctx, msg.Value)
		},
	)
}

// SelfHandler returns the handler for events originating from the Notification service itself.
func (c *NotificationEventConsumer) SelfHandler() *SelfEventHandler {
	return c.selfHandler
}

// ExternalHandler returns the handler for domain events originating from other external services.
func (c *NotificationEventConsumer) ExternalHandler() *ExternalEventHandler {
	return c.externalHandler
}

// ProcessMessage routes CloudEvents to either the SelfEventHandler or ExternalEventHandler.
func (c *NotificationEventConsumer) ProcessMessage(ctx context.Context, msgValue []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(msgValue, &env); err != nil {
		slog.ErrorContext(ctx, "Unmarshal CloudEvent failed", slog.Any("error", err))
		return err
	}

	// 1. Phân luồng sự kiện từ chính Notification Service (Self Events: Push, Email, SMS)
	if c.selfHandler != nil && c.selfHandler.CanHandle(env.Type) {
		return c.selfHandler.Handle(ctx, &env, msgValue)
	}

	// 2. Phân luồng sự kiện từ các Service khác (External Services: Workout, Nutrition, Coaching, ...)
	if c.externalHandler != nil && c.externalHandler.CanHandle(env.Type) {
		return c.externalHandler.Handle(ctx, &env, msgValue)
	}

	// Bỏ qua các sự kiện không nằm trong danh sách đăng ký
	return nil
}
