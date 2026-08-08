package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"google.golang.org/protobuf/encoding/protojson"
)

type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

type UserRegisteredConsumer struct {
	reader        *kafka.Reader
	userRepo      repository.UserProfileRepository
	outboxLogRepo port.OutboxLogRepository
	txManager     port.TransactionManager
}

func NewUserRegisteredConsumer(
	reader *kafka.Reader,
	userRepo repository.UserProfileRepository,
	outboxLogRepo port.OutboxLogRepository,
	txManager port.TransactionManager,
) *UserRegisteredConsumer {
	return &UserRegisteredConsumer{
		reader:        reader,
		userRepo:      userRepo,
		outboxLogRepo: outboxLogRepo,
		txManager:     txManager,
	}
}

func (c *UserRegisteredConsumer) Start(ctx context.Context) {
	log.Println("Starting Profile UserRegistered Kafka Consumer with Outbox Log & 3-Attempt Retry...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Profile UserRegistered Kafka Consumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("Error reading message from auth.events topic: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("Exhausted all retries processing UserRegistered event: %v", err)
			}
		}
	}
}

//nolint:gocritic // kafka.Message is passed by value matching consumer handler contract
func (c *UserRegisteredConsumer) ProcessMessage(ctx context.Context, msg kafka.Message) error {
	var event CloudEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal cloudevent envelope: %w", err)
	}

	if event.Type != "contracts.generic.auth.v1.userRegistered" && event.Type != "UserRegistered" {
		// Ignore unrelated events on topic
		return nil
	}

	log.Printf("Profile UserRegisteredConsumer received event ID: %s, type: %s", event.ID, event.Type)

	// 1. Check Outbox Log Idempotency: skip if already processed
	if c.outboxLogRepo != nil && event.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, event.ID)
		if err != nil {
			log.Printf("Warning: error checking outbox log idempotency: %v", err)
		} else if processed {
			log.Printf("Event %s already processed in outbox_log, skipping (idempotent)", event.ID)
			return nil
		}
	}

	var data authv1event.UserRegistered
	if err := protojson.Unmarshal(event.Data, &data); err != nil {
		// Fallback to standard json unmarshal if payload isn't strict proto json
		if errLegacy := json.Unmarshal(event.Data, &data); errLegacy != nil {
			return fmt.Errorf("unmarshal UserRegistered data payload: %w", err)
		}
	}

	if data.GetUserId() == "" {
		return nil
	}

	// 2. Retry mechanism up to 3 attempts with incremental backoff
	const maxRetries = 3
	var processErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		processErr = c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			// Check if profile already exists in domain repo
			_, findErr := c.userRepo.FindByUserID(txCtx, data.GetUserId())
			if findErr != nil {
				if errors.Is(findErr, derror.ErrProfileNotFound) {
					// Create initial blank profile
					bio, _ := vo.NewBiologicalMetrics(1.0, 1.0, 0, "")
					blankProfile, createErr := aggregate.NewUserProfile(
						data.GetUserId(),
						bio,
						"BEGINNER",
						[]string{},
						[]string{},
						[]string{},
						[]string{},
						"FRIENDLY",
						0,
						0,
						nil,
					)
					if createErr != nil {
						return fmt.Errorf("create blank profile: %w", createErr)
					}
					blankProfile.SetIdentity(data.GetFullName(), data.GetAvatarUrl())
					if saveErr := c.userRepo.Save(txCtx, blankProfile); saveErr != nil {
						return fmt.Errorf("save blank profile: %w", saveErr)
					}
				} else {
					return fmt.Errorf("check profile existence: %w", findErr)
				}
			}

			// Record processed event in outbox_log for idempotency
			if c.outboxLogRepo != nil && event.ID != "" {
				logRecord := &port.OutboxLogRecord{
					ID:           event.ID,
					EventID:      event.ID,
					EventType:    event.Type,
					Payload:      msg.Value,
					PartitionKey: data.GetUserId(),
					Status:       "PROCESSED",
				}
				if logErr := c.outboxLogRepo.SaveLog(txCtx, logRecord); logErr != nil {
					return fmt.Errorf("save outbox log: %w", logErr)
				}
			}

			return nil
		})

		if processErr == nil {
			log.Printf("Successfully processed UserRegistered event on attempt %d/%d for user: %s", attempt, maxRetries, data.GetUserId())
			return nil
		}

		log.Printf("Attempt %d/%d failed to process UserRegistered event for user %s: %v", attempt, maxRetries, data.GetUserId(), processErr)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	// 3. Mark failed event in outbox_log if all retries failed
	if c.outboxLogRepo != nil && event.ID != "" {
		_ = c.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			return c.outboxLogRepo.SaveLog(txCtx, &port.OutboxLogRecord{
				ID:           event.ID,
				EventID:      event.ID,
				EventType:    event.Type,
				Payload:      msg.Value,
				PartitionKey: data.GetUserId(),
				Status:       "FAILED",
				ErrorMessage: processErr.Error(),
			})
		})
	}

	return fmt.Errorf("failed to process UserRegistered event after %d attempts: %w", maxRetries, processErr)
}
