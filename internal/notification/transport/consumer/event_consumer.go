package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
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
	reader          *kafka.Reader
	selfHandler     *SelfEventHandler
	externalHandler *ExternalEventHandler
}

func NewNotificationEventConsumer(
	reader *kafka.Reader,
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

// Start begins consuming Kafka messages continuously until the context is canceled.
func (c *NotificationEventConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		log.Println("[Kafka Consumer] Notification consumer skipping: Kafka reader is nil")
		return
	}

	log.Println("[Kafka Consumer] Notification consumer started, listening for events...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Kafka Consumer] Notification consumer stopped")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Kafka Consumer] Read message error: %v", err)
				continue
			}

			_ = c.ProcessMessage(ctx, msg.Value)
		}
	}
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
		log.Printf("[Kafka Consumer] Unmarshal CloudEvent failed: %v", err)
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
