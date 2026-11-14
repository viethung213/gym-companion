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
	UserID      string `json:"userId"`
	MuscleGroup string `json:"muscleGroup"`
}

// UnmarshalJSON supports both camelCase and snake_case for flexibility.
func (p *ProfileEventPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		UserIDCamel      string `json:"userId"`
		UserIDSnake      string `json:"user_id"`
		MuscleGroupCamel string `json:"muscleGroup"`
		MuscleGroupSnake string `json:"muscle_group"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.UserIDCamel != "" {
		p.UserID = raw.UserIDCamel
	} else {
		p.UserID = raw.UserIDSnake
	}

	if raw.MuscleGroupCamel != "" {
		p.MuscleGroup = raw.MuscleGroupCamel
	} else {
		p.MuscleGroup = raw.MuscleGroupSnake
	}

	return nil
}

// ProfileCompletedConsumer listens to the "profile.events" Kafka topic and
// triggers InitiateRoadmapHandler or RegenerateScheduleHandler when profile events arrive.
// Idempotency is enforced through coaching.outbox_log (D9 pattern).
type ProfileCompletedConsumer struct {
	reader     *segmentio.Reader
	initiate   *command.InitiateRoadmapHandler
	regenerate *command.RegenerateScheduleHandler
	outbox     port.OutboxRepository
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

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping ProfileCompletedConsumer due to context cancellation")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				slog.ErrorContext(ctx, "Error reading profile.events", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second)
				continue
			}

			if processErr := c.processWithRetry(ctx, &msg); processErr != nil {
				slog.ErrorContext(ctx, "Exhausted retries processing profile event", slog.String("error", processErr.Error()))
			}
		}
	}
}

// processWithRetry attempts to handle the message up to maxRetries times with
// incremental backoff (100ms per attempt number).
func (c *ProfileCompletedConsumer) processWithRetry(ctx context.Context, msg *segmentio.Message) error {
	const maxRetries = 3

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = c.HandleMessage(ctx, msg.Value)
		if lastErr == nil {
			return nil
		}

		slog.WarnContext(ctx, "Profile event attempt failed",
			slog.Int("attempt", attempt), slog.Int("max_retries", maxRetries), slog.String("error", lastErr.Error()))
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

	// D9: idempotency check via coaching.outbox_log.
	fresh, err := c.outbox.LogProcessed(ctx, env.ID, env.Type, "", rawValue)
	if err != nil {
		return fmt.Errorf("log processed: %w", err)
	}

	if !fresh {
		slog.DebugContext(ctx, "Duplicate profile event skipped", slog.String("event_id", env.ID))
		return nil
	}

	switch env.Type {
	case profileCompletedEventType, profileCompletedEventTypeShort:
		return c.handleProfileCompleted(ctx, env.Data)

	case profileUpdatedEventType, profileUpdatedEventTypeShort:
		return c.handleProfileUpdated(ctx, env.Data)

	case injuryReportedEventType, injuryReportedEventTypeShort:
		return c.handleInjuryReported(ctx, env.Data)

	case injuryRecoveredEventType, injuryRecoveredEventTypeShort:
		return c.handleInjuryRecovered(ctx, env.Data)

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

func (c *ProfileCompletedConsumer) handleProfileUpdated(ctx context.Context, data []byte) error {
	if c.regenerate == nil {
		return nil
	}

	var p ProfileEventPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode ProfileUpdated payload: %w", err)
	}

	if p.UserID == "" {
		return nil
	}

	slog.InfoContext(ctx, "ProfileUpdated received, regenerating pending schedule", slog.String("user_id", p.UserID))
	_, err := c.regenerate.Handle(ctx, command.RegenerateScheduleCommand{
		UserID: p.UserID,
		Reason: "profile_updated",
	})
	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return fmt.Errorf("regenerate schedule on profile updated for user %s: %w", p.UserID, err)
	}
	return nil
}

func (c *ProfileCompletedConsumer) handleInjuryReported(ctx context.Context, data []byte) error {
	if c.regenerate == nil {
		return nil
	}

	var p ProfileEventPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode InjuryReported payload: %w", err)
	}

	if p.UserID == "" {
		return nil
	}

	slog.InfoContext(ctx, "InjuryReported received, regenerating schedule", slog.String("user_id", p.UserID), slog.String("muscle_group", p.MuscleGroup))
	_, err := c.regenerate.Handle(ctx, command.RegenerateScheduleCommand{
		UserID: p.UserID,
		Reason: "injury_reported",
	})
	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return fmt.Errorf("regenerate schedule on injury reported for user %s: %w", p.UserID, err)
	}
	return nil
}

func (c *ProfileCompletedConsumer) handleInjuryRecovered(ctx context.Context, data []byte) error {
	if c.regenerate == nil {
		return nil
	}

	var p ProfileEventPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode InjuryRecovered payload: %w", err)
	}

	if p.UserID == "" {
		return nil
	}

	slog.InfoContext(ctx, "InjuryRecovered received, regenerating schedule", slog.String("user_id", p.UserID))
	_, err := c.regenerate.Handle(ctx, command.RegenerateScheduleCommand{
		UserID: p.UserID,
		Reason: "injury_recovered",
	})
	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return fmt.Errorf("regenerate schedule on injury recovered for user %s: %w", p.UserID, err)
	}
	return nil
}
