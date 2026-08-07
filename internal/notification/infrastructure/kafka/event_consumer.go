package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

type NotificationEventConsumer struct {
	reader          *kafka.Reader
	sendPushHandler *command.SendPushNotificationHandler
	outboxLogRepo   port.OutboxLogRepository
}

type cloudEventEnvelope struct {
	ID     string          `json:"id"`
	Source string          `json:"source"`
	Type   string          `json:"type"`
	Time   string          `json:"time"`
	Data   json.RawMessage `json:"data"`
}

type defaultEventData struct {
	UserID  string `json:"userId"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Body    string `json:"body"`
}

type eventPushConfig struct {
	DefaultTitle string
	DefaultBody  string
}

// pushAllowedEventRules maps allowed CloudEvent types to their default Push title and body formatting rules.
// STRICT RULE: Only the 3 explicit Core Events (Workout PR, Nutrition Meal Reminder -30m, Coaching Workout Reminder -1h)
// and Notification Standard Event (NotificationRequested) are listened and dispatched for Push Notifications.
var pushAllowedEventRules = map[string]eventPushConfig{
	// =========================================================================
	// 1. STANDARD NOTIFICATION CONTRACT EVENT (Dành cho TẤT CẢ các module khác)
	// =========================================================================
	"contracts.generic.notification.v1.event.NotificationRequested": {
		DefaultTitle: "Gym Companion Thông Báo",
		DefaultBody:  "Bạn có thông báo mới từ ứng dụng.",
	},

	// =========================================================================
	// 2. CORE MODULE: WORKOUT EXECUTION (Lập Kỷ kỷ lục cá nhân mới - PR)
	// =========================================================================
	"contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved": {
		DefaultTitle: "Kỷ kỷ lục cá nhân mới! 🏆",
		DefaultBody:  "Chúc mừng bạn vừa xác lập một kỷ kỷ lục cá nhân (PR) mới!",
	},

	// =========================================================================
	// 3. CORE MODULE: NUTRITION (Sắp đến bữa ăn - Trước 30 phút)
	// =========================================================================
	"contracts.core.nutrition.v1.event.UpcomingMealReminder": {
		DefaultTitle: "Nhắc nhở bữa ăn 🥗",
		DefaultBody:  "Sắp đến giờ ăn theo lịch dinh dưỡng (trước 30 phút). Nhớ chuẩn bị bữa ăn nhé!",
	},

	// =========================================================================
	// 4. CORE MODULE: COACHING (Sắp đến giờ tập - Trước 1 tiếng)
	// =========================================================================
	"contracts.core.coaching.v1.event.UpcomingWorkoutReminder": {
		DefaultTitle: "Nhắc nhở buổi tập ⏰",
		DefaultBody:  "Sắp đến giờ tập luyện theo lịch HLV (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!",
	},
}

func NewNotificationEventConsumer(
	reader *kafka.Reader,
	sendPushHandler *command.SendPushNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *NotificationEventConsumer {
	return &NotificationEventConsumer{
		reader:          reader,
		sendPushHandler: sendPushHandler,
		outboxLogRepo:   outboxLogRepo,
	}
}

func (c *NotificationEventConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		log.Println("[Kafka Consumer] Notification consumer skipping: Kafka reader is nil")
		return
	}

	log.Println("[Kafka Consumer] Notification consumer started, listening ONLY for 3 Core Events & Notification Standard Event...")
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

func (c *NotificationEventConsumer) ProcessMessage(ctx context.Context, msgValue []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(msgValue, &env); err != nil {
		log.Printf("[Kafka Consumer] Unmarshal CloudEvent failed: %v", err)
		return err
	}

	// 1. FILTER RULE: Check if event type is one of the allowed core events or generic notification event
	rule, isAllowed := pushAllowedEventRules[env.Type]
	if !isAllowed {
		// All other events are strictly ignored
		return nil
	}

	var payload defaultEventData
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		log.Printf("[Kafka Consumer] Unmarshal CloudEvent data failed for '%s': %v", env.Type, err)
		return err
	}

	if payload.UserID == "" {
		return nil
	}

	// 2. IDEMPOTENCY CHECK via notification.outbox_log
	if c.outboxLogRepo != nil && env.ID != "" {
		fresh, logErr := c.outboxLogRepo.LogProcessed(ctx, env.ID, env.Type, payload.UserID, msgValue, "PROCESSING", "")
		if logErr != nil {
			log.Printf("[Kafka Consumer] LogProcessed error for event '%s': %v", env.ID, logErr)
		} else if !fresh {
			// Duplicate event detected, skip to avoid double push!
			log.Printf("[Kafka Consumer] Duplicate event '%s' (ID: %s) already recorded in notification.outbox_log, skipping...", env.Type, env.ID)
			return nil
		}
	}

	// 3. Format Title & Body based on payload or default rule template
	title := payload.Title
	if title == "" {
		title = rule.DefaultTitle
	}

	body := payload.Body
	if body == "" {
		body = payload.Message
	}
	if body == "" {
		body = rule.DefaultBody
	}

	log.Printf("[Kafka Consumer] Targeted Event '%s' matched for user '%s', dispatching push...", env.Type, payload.UserID)
	var sendErr error
	if c.sendPushHandler != nil {
		_, sendErr = c.sendPushHandler.Handle(ctx, command.SendPushNotificationCommand{
			UserID: payload.UserID,
			Title:  title,
			Body:   body,
			Data: map[string]string{
				"eventId":   env.ID,
				"eventType": env.Type,
				"source":    env.Source,
			},
		})
	}

	if sendErr != nil {
		log.Printf("[Kafka Consumer] Error sending push notification for event '%s': %v", env.ID, sendErr)
		if c.outboxLogRepo != nil && env.ID != "" {
			_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      env.ID,
				EventType:    env.Type,
				Payload:      msgValue,
				PartitionKey: payload.UserID,
				Status:       "FAILED",
				ErrorMessage: sendErr.Error(),
			})
		}
		return sendErr
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      msgValue,
			PartitionKey: payload.UserID,
			Status:       "SUCCESS",
			ErrorMessage: "",
		})
	}

	return nil
}
