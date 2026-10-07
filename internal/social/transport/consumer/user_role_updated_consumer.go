package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"google.golang.org/protobuf/encoding/protojson"
)

type UserRoleUpdatedConsumer struct {
	reader            *kafka.Reader
	updateRoleHandler *command.UpdateUserRoleHandler
	outboxLogRepo     port.OutboxLogRepository
}

func NewUserRoleUpdatedConsumer(
	reader *kafka.Reader,
	updateRoleHandler *command.UpdateUserRoleHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserRoleUpdatedConsumer {
	return &UserRoleUpdatedConsumer{
		reader:            reader,
		updateRoleHandler: updateRoleHandler,
		outboxLogRepo:     outboxLogRepo,
	}
}

func (c *UserRoleUpdatedConsumer) Start(ctx context.Context) {
	log.Println("Starting Social UserRoleUpdated Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social UserRoleUpdated Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Social UserRoleUpdated consumer read error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("Social UserRoleUpdated consumer process error: %v", err)
			}
		}
	}
}

//nolint:gocritic // kafka.Message is passed by value matching consumer handler contract
func (c *UserRoleUpdatedConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var cloudEvent CloudEvent
	if err := json.Unmarshal(msg.Value, &cloudEvent); err != nil {
		return fmt.Errorf("unmarshal cloud event envelope: %w", err)
	}

	if cloudEvent.Type != "contracts.generic.auth.v1.userRoleUpdated" {
		return nil
	}

	// Idempotency check
	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, cloudEvent.ID)
		if err == nil && processed {
			return nil
		}
	}

	var event authv1event.UserRoleUpdated
	if err := protojson.Unmarshal(cloudEvent.Data, &event); err != nil {
		return fmt.Errorf("unmarshal UserRoleUpdated data: %w", err)
	}

	err := c.handleEvent(ctx, cloudEvent.ID, &event)
	if err != nil {
		if c.outboxLogRepo != nil && cloudEvent.ID != "" {
			_ = c.outboxLogRepo.Save(ctx, cloudEvent.ID, cloudEvent.Type, msg.Value, string(msg.Key), "FAILED", err.Error())
		}
		return err
	}

	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		_ = c.outboxLogRepo.Save(ctx, cloudEvent.ID, cloudEvent.Type, msg.Value, string(msg.Key), "PROCESSED", "")
	}

	return nil
}

func (c *UserRoleUpdatedConsumer) handleEvent(ctx context.Context, _ string, event *authv1event.UserRoleUpdated) error {
	return c.updateRoleHandler.Handle(ctx, command.UpdateUserRoleCommand{
		UserID: event.GetUserId(),
		Role:   event.GetNewRole(),
	})
}
