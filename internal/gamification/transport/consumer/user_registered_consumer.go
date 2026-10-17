package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
)

// UserRegisteredConsumer initializes a fresh baseline profile when a user registers.
type UserRegisteredConsumer struct {
	reader        *kafka.Reader
	initHandler   *command.InitUserGamificationHandler
	outboxLogRepo port.OutboxLogRepository
}

// NewUserRegisteredConsumer constructs a new UserRegisteredConsumer.
func NewUserRegisteredConsumer(
	reader *kafka.Reader,
	initHandler *command.InitUserGamificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserRegisteredConsumer {
	return &UserRegisteredConsumer{
		reader:        reader,
		initHandler:   initHandler,
		outboxLogRepo: outboxLogRepo,
	}
}

// Start polls Kafka for UserRegistered events until context cancellation.
func (c *UserRegisteredConsumer) Start(ctx context.Context) {
	if c.reader == nil {
		return
	}
	log.Println("[Gamification] Starting UserRegistered Kafka Consumer...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping UserRegistered Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Gamification] UserRegistered consumer fetch error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = c.ProcessMessage(ctx, &msg)
				if processErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
			}

			if processErr != nil {
				log.Printf("[Gamification] UserRegistered consumer process error after retries: %v", processErr)
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Gamification] UserRegistered consumer commit error: %v", err)
			}
		}
	}
}

// ProcessMessage handles deserialization, idempotency, and command dispatch.
func (c *UserRegisteredConsumer) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return errors.New("kafka message is nil")
	}
	var ce CloudEvent
	if err := json.Unmarshal(msg.Value, &ce); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if ce.Type != "contracts.generic.auth.v1.userRegistered" {
		return nil
	}

	// Consumer Idempotency Check
	if c.outboxLogRepo != nil && ce.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, ce.ID)
		if err == nil && processed {
			return nil
		}
	}

	var protoEv authv1event.UserRegistered
	if err := protojson.Unmarshal(ce.Data, &protoEv); err != nil {
		return fmt.Errorf("unmarshal proto UserRegistered: %w", err)
	}

	cmd := command.InitUserGamificationCommand{
		UserID:      protoEv.GetUserId(),
		DisplayName: protoEv.GetFullName(),
		AvatarURL:   protoEv.GetAvatarUrl(),
	}

	err := c.initHandler.Handle(ctx, cmd)
	if err != nil {
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
