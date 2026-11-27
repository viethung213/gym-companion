package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"google.golang.org/protobuf/encoding/protojson"
)

type UserIdentityUpdatedConsumer struct {
	reader        *kafka.Reader
	dlqWriter     sharedKafka.MessageWriter
	syncHandler   *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func (c *UserIdentityUpdatedConsumer) WithDLQWriter(writer sharedKafka.MessageWriter) *UserIdentityUpdatedConsumer {
	c.dlqWriter = writer
	return c
}

func NewUserIdentityUpdatedConsumer(
	reader *kafka.Reader,
	syncHandler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserIdentityUpdatedConsumer {
	return &UserIdentityUpdatedConsumer{
		reader:        reader,
		syncHandler:   syncHandler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserIdentityUpdatedConsumer) Start(ctx context.Context) {
	slog.InfoContext(ctx, "Starting Social UserIdentityUpdated Kafka Consumer")
	sharedKafka.RunConsumerLoop(
		ctx,
		c.reader,
		sharedKafka.ConsumerConfig{
			Name:        "SocialUserIdentityUpdatedConsumer",
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   c.dlqWriter,
		},
		func(ctx context.Context, msg *kafka.Message) error {
			return c.ProcessMessage(ctx, *msg)
		},
	)
}

//nolint:gocritic // kafka.Message is passed by value matching consumer handler contract
func (c *UserIdentityUpdatedConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var cloudEvent CloudEvent
	if err := json.Unmarshal(msg.Value, &cloudEvent); err != nil {
		return fmt.Errorf("unmarshal cloud event envelope: %w", err)
	}

	if cloudEvent.Type != "contracts.supporting.profile.v1.event.UserIdentityUpdated" {
		return nil
	}

	// Idempotency check
	if c.outboxLogRepo != nil && cloudEvent.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, cloudEvent.ID)
		if err == nil && processed {
			return nil
		}
	}

	var event profilev1event.UserIdentityUpdated
	if err := protojson.Unmarshal(cloudEvent.Data, &event); err != nil {
		return fmt.Errorf("unmarshal UserIdentityUpdated data: %w", err)
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

func (c *UserIdentityUpdatedConsumer) handleEvent(ctx context.Context, _ string, event *profilev1event.UserIdentityUpdated) error {
	return c.syncHandler.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:    event.GetUserId(),
		FullName:  event.GetFullName(),
		AvatarURL: event.GetAvatarUrl(),
	})
}
