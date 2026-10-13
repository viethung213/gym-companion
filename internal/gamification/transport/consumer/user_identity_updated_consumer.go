package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

// UserIdentityUpdatedConsumer listens to profile updates to keep leaderboard snapshots fresh.
type UserIdentityUpdatedConsumer struct {
	reader        *kafka.Reader
	syncHandler   *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

// NewUserIdentityUpdatedConsumer constructs UserIdentityUpdatedConsumer.
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

// Start polls Kafka for UserIdentityUpdated events until context cancellation.
func (c *UserIdentityUpdatedConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting UserIdentityUpdated Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping UserIdentityUpdated Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] UserIdentityUpdated consumer read error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("[Gamification] UserIdentityUpdated consumer process error: %v", err)
			}
		}
	}
}

// ProcessMessage deserializes event, verifies idempotency, and updates snapshot.
//
//nolint:gocritic // kafka.Message parameter matches signature
func (c *UserIdentityUpdatedConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if ce.Type != "contracts.supporting.profile.v1.event.UserIdentityUpdated" {
		return nil
	}

	// Idempotency check via outbox_log
	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if err == nil && processed {
			return nil
		}
	}

	var protoEv profilev1event.UserIdentityUpdated
	if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
		return fmt.Errorf("unmarshal proto UserIdentityUpdated: %w", err)
	}

	cmd := command.SyncUserSnapshotCommand{
		UserID:      protoEv.GetUserId(),
		DisplayName: protoEv.GetFullName(),
		AvatarURL:   protoEv.GetAvatarUrl(),
	}

	if err := c.syncHandler.Handle(ctx, cmd); err != nil {
		if c.outboxLogRepo != nil && ce.ID != "" {
			_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      ce.ID,
				EventType:    ce.Type,
				Payload:      msg.Value,
				PartitionKey: string(msg.Key),
				Status:       "FAILED",
				ErrorMessage: err.Error(),
			})
		}
		return err
	}

	if c.outboxLogRepo != nil && ce.ID != "" {
		_ = c.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      ce.ID,
			EventType:    ce.Type,
			Payload:      msg.Value,
			PartitionKey: string(msg.Key),
			Status:       "PROCESSED",
		})
	}

	return nil
}
