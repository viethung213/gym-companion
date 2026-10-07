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

type UserRegisteredConsumer struct {
	reader        *kafka.Reader
	syncHandler   *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewUserRegisteredConsumer(
	reader *kafka.Reader,
	syncHandler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserRegisteredConsumer {
	return &UserRegisteredConsumer{
		reader:        reader,
		syncHandler:   syncHandler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserRegisteredConsumer) Start(ctx context.Context) {
	log.Println("Starting Social UserRegistered Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social UserRegistered Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Social UserRegistered consumer read error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("Social UserRegistered consumer process error: %v", err)
			}
		}
	}
}

//nolint:gocritic // kafka.Message is passed by value matching consumer handler contract
func (c *UserRegisteredConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var cloudEvent CloudEvent
	if err := json.Unmarshal(msg.Value, &cloudEvent); err != nil {
		return fmt.Errorf("unmarshal cloud event envelope: %w", err)
	}

	if cloudEvent.Type != "contracts.generic.auth.v1.userRegistered" {
		return nil
	}

	// Idempotency check
	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, cloudEvent.ID)
		if err == nil && processed {
			return nil
		}
	}

	var event authv1event.UserRegistered
	if err := protojson.Unmarshal(cloudEvent.Data, &event); err != nil {
		return fmt.Errorf("unmarshal UserRegistered data: %w", err)
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

func (c *UserRegisteredConsumer) handleEvent(ctx context.Context, _ string, event *authv1event.UserRegistered) error {
	role := event.GetRole()
	if role == "" {
		role = "user"
	}
	return c.syncHandler.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:    event.GetUserId(),
		FullName:  event.GetFullName(),
		AvatarURL: event.GetAvatarUrl(),
		Role:      role,
	})
}
