// Package consumer routes inbound events (Kafka CloudEvents) to command handlers.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// CloudEvent types emitted by the Profile module.
const (
	profileCompletedEventType      = "contracts.supporting.profile.v1.event.ProfileCompleted"
	profileCompletedEventTypeShort = "contracts.supporting.profile.v1.profileCompleted"

	profileUpdatedEventType      = "contracts.supporting.profile.v1.event.ProfileUpdated"
	profileUpdatedEventTypeShort = "contracts.supporting.profile.v1.profileUpdated"

	injuryReportedEventType      = "contracts.supporting.profile.v1.event.InjuryReported"
	injuryReportedEventTypeShort = "contracts.supporting.profile.v1.injuryReported"

	injuryRecoveredEventType      = "contracts.supporting.profile.v1.event.InjuryRecovered"
	injuryRecoveredEventTypeShort = "contracts.supporting.profile.v1.injuryRecovered"
)

// ProfileEventPayload mirrors the data field of Profile CloudEvents.
type ProfileEventPayload struct {
	UserID string `json:"userId"`
}

// UnmarshalJSON supports both camelCase and snake_case for flexibility.
func (p *ProfileEventPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		UserIDCamel string `json:"userId"`
		UserIDSnake string `json:"user_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.UserIDCamel != "" {
		p.UserID = raw.UserIDCamel
	} else {
		p.UserID = raw.UserIDSnake
	}

	return nil
}

// ProfileCompletedConsumer listens to the "profile.events" Kafka topic and
// triggers InitiateRoadmapHandler or RegenerateScheduleHandler when profile events arrive.
// Idempotency is enforced through coaching.outbox_log (D9 pattern).
type ProfileCompletedConsumer struct {
	reader     *segmentio.Reader
	dlqWriter  sharedKafka.MessageWriter
	initiate   *command.InitiateRoadmapHandler
	regenerate *command.RegenerateScheduleHandler
	outbox     port.OutboxRepository
}

// WithDLQWriter attaches an optional dead letter queue writer.
func (c *ProfileCompletedConsumer) WithDLQWriter(writer sharedKafka.MessageWriter) *ProfileCompletedConsumer {
	c.dlqWriter = writer
	return c
}

// NewProfileCompletedConsumer wires the consumer.
func NewProfileCompletedConsumer(
	reader *segmentio.Reader,
	initiate *command.InitiateRoadmapHandler,
	regenerate *command.RegenerateScheduleHandler,
	outbox port.OutboxRepository,
) *ProfileCompletedConsumer {
	return &ProfileCompletedConsumer{
		reader:     reader,
		initiate:   initiate,
		regenerate: regenerate,
		outbox:     outbox,
	}
}

// Start begins the blocking read loop. It stops when ctx is cancelled.
func (c *ProfileCompletedConsumer) Start(ctx context.Context) {
	slog.InfoContext(ctx, "Starting ProfileCompletedConsumer on topic profile.events")

	sharedKafka.RunConsumerLoop(
		ctx,
		c.reader,
		sharedKafka.ConsumerConfig{
			Name:        "ProfileCompletedConsumer",
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   c.dlqWriter,
		},
		func(ctx context.Context, msg *segmentio.Message) error {
			return c.HandleMessage(ctx, msg.Value)
		},
	)
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

	return handleOnce(ctx, c.outbox, &env, rawValue, func() error { return c.dispatch(ctx, &env) })
}

// dispatch routes one decoded profile event to its handler.
func (c *ProfileCompletedConsumer) dispatch(ctx context.Context, env *cloudEventEnvelope) error {
	switch env.Type {
	case profileCompletedEventType, profileCompletedEventTypeShort:
		return c.handleProfileCompleted(ctx, env.Data)

	case profileUpdatedEventType, profileUpdatedEventTypeShort:
		return c.regenerateOn(ctx, env, "profile_updated")

	case injuryReportedEventType, injuryReportedEventTypeShort:
		return c.regenerateOn(ctx, env, "injury_reported")

	case injuryRecoveredEventType, injuryRecoveredEventTypeShort:
		return c.regenerateOn(ctx, env, "injury_recovered")

	default:
		slog.DebugContext(ctx, "ProfileCompletedConsumer ignoring event type", slog.String("event_type", env.Type))
		return nil
	}
}

func (c *ProfileCompletedConsumer) handleProfileCompleted(ctx context.Context, data []byte) error {
	if c.initiate == nil {
		return nil
	}

	var p ProfileEventPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode ProfileCompleted payload: %w", err)
	}

	if p.UserID == "" {
		slog.WarnContext(ctx, "ProfileCompleted event missing user_id; ignored")
		return nil
	}

	slog.InfoContext(ctx, "ProfileCompleted received, initiating roadmap", slog.String("user_id", p.UserID))

	_, err := c.initiate.Handle(ctx, command.InitiateRoadmapCommand{UserID: p.UserID})
	if err != nil {
		if errors.Is(err, roadmap.ErrActiveRoadmapExists) {
			slog.InfoContext(ctx, "User already has an active roadmap, skipping auto-initiation", slog.String("user_id", p.UserID))
			return nil
		}

		return fmt.Errorf("initiate roadmap for user %s: %w", p.UserID, err)
	}

	slog.InfoContext(ctx, "Successfully auto-initiated roadmap", slog.String("user_id", p.UserID))
	return nil
}

// regenerateOn rewrites the athlete's pending sessions after a profile change.
// The event time travels with the command: a change made before the active
// roadmap was planned (an injury entered at onboarding) is already in it.
func (c *ProfileCompletedConsumer) regenerateOn(ctx context.Context, env *cloudEventEnvelope, reason string) error {
	if c.regenerate == nil {
		return nil
	}

	var p ProfileEventPayload
	if err := json.Unmarshal(env.Data, &p); err != nil {
		return fmt.Errorf("decode %s payload: %w", env.Type, err)
	}

	if p.UserID == "" {
		return nil
	}

	slog.InfoContext(ctx, "Profile change received, regenerating pending schedule",
		slog.String("user_id", p.UserID), slog.String("reason", reason))
	_, err := c.regenerate.Handle(ctx, command.RegenerateScheduleCommand{
		UserID:    p.UserID,
		Reason:    reason,
		ChangedAt: env.occurredAt(),
	})
	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return fmt.Errorf("regenerate schedule on %s for user %s: %w", reason, p.UserID, err)
	}
	return nil
}
