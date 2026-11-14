package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"google.golang.org/protobuf/encoding/protojson"
)

type UserIdentityUpdatedConsumer struct {
	reader        *kafka.Reader
	syncHandler   *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
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
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Social UserIdentityUpdated Kafka Consumer due to context cancellation")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.ErrorContext(ctx, "Social UserIdentityUpdated consumer read error", slog.String("error", err.Error()))
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				slog.ErrorContext(ctx, "Social UserIdentityUpdated consumer process error", slog.String("error", err.Error()))
			}
		}
	}
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
