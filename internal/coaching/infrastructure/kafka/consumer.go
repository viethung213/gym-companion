package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/event"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

const profileCompletedEventType = "contracts.supporting.profile.v1.event.ProfileCompleted"

type profileCompletedMessagePayload struct {
	UserID                 string   `json:"user_id"`
	Goals                  []string `json:"goals"`
	RegisteredInjuries     []string `json:"registered_injuries"`
	PreferredWorkoutTimes []string `json:"preferred_workout_times"`
	ExperienceLevel        string   `json:"experience_level"`
	CompletedAt            string   `json:"completed_at"`
}

type Consumer struct {
	reader       *kafka.Reader
	inboxRepo    port.InboxRepository
	eventHandler *event.ProfileCompletedHandler
}

func NewConsumer(
	brokers []string,
	topic string,
	groupID string,
	inboxRepo port.InboxRepository,
	eventHandler *event.ProfileCompletedHandler,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10,
		MaxBytes:       10 * 1024 * 1024,
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.FirstOffset,
	})

	return &Consumer{
		reader:       reader,
		inboxRepo:    inboxRepo,
		eventHandler: eventHandler,
	}
}

// Start begins polling messages from Kafka until context cancellation.
func (c *Consumer) Start(ctx context.Context) {
	log.Println("INFO: starting Coaching Kafka consumer for topic:", c.reader.Config().Topic)
	for {
		select {
		case <-ctx.Done():
			log.Println("INFO: stopping Coaching Kafka consumer")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("ERROR: fetch kafka message: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				log.Printf("ERROR: process kafka message (offset %d): %v", msg.Offset, err)
			} else {
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("ERROR: commit kafka message offset %d: %v", msg.Offset, err)
				}
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	eventID := extractHeader(msg, "event_id")
	eventType := extractHeader(msg, "event_type")

	if eventID == "" {
		eventID = fmt.Sprintf("%s-%d", msg.Topic, msg.Offset)
	}

	// Filter: only handle ProfileCompleted events
	if eventType != "" && eventType != profileCompletedEventType {
		return nil
	}

	// Idempotency check: skip if already processed in outbox_log
	processed, err := c.inboxRepo.IsProcessed(ctx, eventID)
	if err != nil {
		return fmt.Errorf("check inbox processed: %w", err)
	}
	if processed {
		log.Printf("INFO: event %s already processed, skipping", eventID)
		return nil
	}

	var payload profileCompletedMessagePayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("unmarshal profile completed payload: %w", err)
	}

	evt := event.ProfileCompletedEvent{
		UserID:             payload.UserID,
		Goals:              payload.Goals,
		RegisteredInjuries: payload.RegisteredInjuries,
		ExperienceLevel:    payload.ExperienceLevel,
	}

	if err := c.eventHandler.Handle(ctx, evt); err != nil {
		return fmt.Errorf("handle profile completed event: %w", err)
	}

	// Mark processed in outbox_log (inbox pattern)
	if err := c.inboxRepo.MarkProcessed(ctx, eventID, eventType, msg.Value, string(msg.Key)); err != nil {
		log.Printf("WARN: failed to mark event %s processed in inbox log: %v", eventID, err)
	}

	return nil
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close kafka reader: %w", err)
	}
	return nil
}

func extractHeader(msg kafka.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
