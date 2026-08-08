// Package consumer routes inbound events (Kafka CloudEvents) to command handlers.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// profileCompletedEventType is the CloudEvent type emitted by the Profile
// module when a user finishes onboarding.
const profileCompletedEventType = "contracts.supporting.profile.v1.event.ProfileCompleted"

// ProfileCompletedPayload mirrors the data field of a ProfileCompleted
// CloudEvent. Only the fields Coaching actually needs are decoded.
type ProfileCompletedPayload struct {
	UserID string `json:"userId"`
}

// ProfileCompletedConsumer listens to the "profile.events" Kafka topic and
// triggers InitiateRoadmapHandler when a ProfileCompleted event arrives.
// Idempotency is enforced through coaching.outbox_log (D9 pattern).
type ProfileCompletedConsumer struct {
	reader   *segmentio.Reader
	initiate *command.InitiateRoadmapHandler
	outbox   port.OutboxRepository
}

// NewProfileCompletedConsumer wires the consumer.
func NewProfileCompletedConsumer(
	reader *segmentio.Reader,
	initiate *command.InitiateRoadmapHandler,
	outbox port.OutboxRepository,
) *ProfileCompletedConsumer {
	return &ProfileCompletedConsumer{
		reader:   reader,
		initiate: initiate,
		outbox:   outbox,
	}
}

// Start begins the blocking read loop. It stops when ctx is cancelled.
func (c *ProfileCompletedConsumer) Start(ctx context.Context) {
	log.Println("[Coaching] Starting ProfileCompletedConsumer on topic profile.events...")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Coaching] Stopping ProfileCompletedConsumer due to context cancellation.")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				log.Printf("[Coaching] Error reading profile.events: %v", err)

				time.Sleep(1 * time.Second)

				continue
			}

			if processErr := c.processWithRetry(ctx, msg); processErr != nil {
				log.Printf("[Coaching] Exhausted retries processing ProfileCompleted event: %v", processErr)
			}
		}
	}
}

// processWithRetry attempts to handle the message up to maxRetries times with
// incremental backoff (100ms per attempt number).
func (c *ProfileCompletedConsumer) processWithRetry(ctx context.Context, msg segmentio.Message) error {
	const maxRetries = 3

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = c.HandleMessage(ctx, msg.Value)
		if lastErr == nil {
			return nil
		}

		log.Printf("[Coaching] ProfileCompleted attempt %d/%d failed: %v", attempt, maxRetries, lastErr)

		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// HandleMessage decodes and dispatches one Kafka message. Exported so that
// unit tests can exercise the logic without a running Kafka broker.
func (c *ProfileCompletedConsumer) HandleMessage(ctx context.Context, rawValue []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawValue, &env); err != nil {
		return fmt.Errorf("decode cloud event: %w", err)
	}

	if env.ID == "" {
		return errors.New("missing event id")
	}

	// Only handle ProfileCompleted; ignore other events on the same topic.
	if env.Type != profileCompletedEventType {
		log.Printf("[Coaching] ProfileCompletedConsumer ignoring event type: %s", env.Type)

		return nil
	}

	// D9: idempotency check via coaching.outbox_log.
	fresh, err := c.outbox.LogProcessed(ctx, env.ID, env.Type, "", rawValue)
	if err != nil {
		return fmt.Errorf("log processed: %w", err)
	}

	if !fresh {
		log.Printf("[Coaching] Duplicate ProfileCompleted event skipped: id=%s", env.ID)

		return nil
	}

	return c.handleProfileCompleted(ctx, env.Data)
}

func (c *ProfileCompletedConsumer) handleProfileCompleted(ctx context.Context, data []byte) error {
	var p ProfileCompletedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode ProfileCompleted payload: %w", err)
	}

	if p.UserID == "" {
		log.Println("[Coaching] ProfileCompleted event missing user_id; ignored")

		return nil
	}

	log.Printf("[Coaching] ProfileCompleted received for user %s — initiating roadmap", p.UserID)

	_, err := c.initiate.Handle(ctx, command.InitiateRoadmapCommand{UserID: p.UserID})
	if err != nil {
		// If the user already has an active roadmap, this is expected and not
		// an error worth retrying.
		if errors.Is(err, roadmap.ErrActiveRoadmapExists) {
			log.Printf("[Coaching] User %s already has an active roadmap; skipping auto-initiation", p.UserID)

			return nil
		}

		return fmt.Errorf("initiate roadmap for user %s: %w", p.UserID, err)
	}

	log.Printf("[Coaching] Successfully auto-initiated roadmap for user %s", p.UserID)

	return nil
}
