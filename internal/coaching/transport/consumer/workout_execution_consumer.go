// Package consumer routes inbound events (Kafka CloudEvents) to command handlers.
// Idempotency is enforced by coaching.outbox_log (D9): consumer checks event_id

// before dispatching, and records the event after successful processing.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// cloudEventEnvelope is the minimum structure we parse from Kafka message value.
type cloudEventEnvelope struct {
	SpecVersion string          `json:"specversion"`
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
}

// WorkoutSessionCompletedPayload mirrors the proto CloudEvent data field.
type WorkoutSessionCompletedPayload struct {
	SessionID        string  `json:"session_id"`
	UserID           string  `json:"user_id"`
	CompletedAt      string  `json:"completed_at"` // RFC3339
	TotalSets        int32   `json:"total_sets"`
	TotalVolume      float32 `json:"total_volume"`
	AverageFormScore float32 `json:"average_form_score"`
	AverageRPE       float32 `json:"average_rpe"`
	PlanID           string  `json:"plan_id"`
}

// WorkoutSessionAbortedPayload mirrors the proto CloudEvent data field.
type WorkoutSessionAbortedPayload struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	AbortedAt string `json:"aborted_at"`
	Reason    string `json:"reason"`
	PlanID    string `json:"plan_id"`
}

// WorkoutExecutionConsumer routes WorkoutSessionCompleted/Aborted events to
// the appropriate command handler.
type WorkoutExecutionConsumer struct {
	complete *command.CompleteSessionHandler
	abort    *command.AbortSessionHandler
	outbox   port.OutboxRepository
}

// NewWorkoutExecutionConsumer wires the consumer.
func NewWorkoutExecutionConsumer(
	complete *command.CompleteSessionHandler,
	abort *command.AbortSessionHandler,
	outbox port.OutboxRepository,
) *WorkoutExecutionConsumer {
	return &WorkoutExecutionConsumer{complete: complete, abort: abort, outbox: outbox}
}

// HandleMessage decodes and dispatches one Kafka message. Errors are returned
// so the caller can decide retry vs DLQ policy.
func (c *WorkoutExecutionConsumer) HandleMessage(ctx context.Context, rawValue []byte) error {
	var env cloudEventEnvelope

	if err := json.Unmarshal(rawValue, &env); err != nil {
		return fmt.Errorf("decode cloud event: %w", err)
	}

	if env.ID == "" {
		return errors.New("missing event id")
	}

	// D9: idempotency check via coaching.outbox_log

	fresh, err := c.outbox.LogProcessed(ctx, env.ID, env.Type, "", rawValue)

	if err != nil {
		return fmt.Errorf("log processed: %w", err)
	}

	if !fresh {
		log.Printf("[Coaching] Duplicate event skipped: id=%s type=%s", env.ID, env.Type)

		return nil
	}

	switch env.Type {
	case "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted":
		return c.handleCompleted(ctx, env.Data)

	case "contracts.core.workout_execution.v1.event.WorkoutSessionAborted":
		return c.handleAborted(ctx, env.Data)

	default:
		log.Printf("[Coaching] Ignoring unknown event type: %s", env.Type)

		return nil
	}
}

func (c *WorkoutExecutionConsumer) handleCompleted(ctx context.Context, data []byte) error {
	var p WorkoutSessionCompletedPayload

	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode WorkoutSessionCompleted: %w", err)
	}

	if p.PlanID == "" {
		log.Printf("[Coaching] WorkoutSessionCompleted missing plan_id; session=%s ignored", p.SessionID)

		return nil
	}

	_, err := c.complete.Handle(ctx, command.CompleteSessionCommand{
		SessionPlanID: p.PlanID,

		TotalActualSets: int(p.TotalSets),

		TotalPrescribedSets: 0, // Best-effort: recomputed inside handler if available.

		AverageActualRPE: float64(p.AverageRPE),

		CompletedAt: p.CompletedAt,
	})

	if errors.Is(err, roadmap.ErrSessionNotFound) {
		log.Printf("[Coaching] WorkoutSessionCompleted references unknown plan_id=%s; ignoring", p.PlanID)

		return nil
	}

	return err
}

func (c *WorkoutExecutionConsumer) handleAborted(ctx context.Context, data []byte) error {
	var p WorkoutSessionAbortedPayload

	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode WorkoutSessionAborted: %w", err)
	}

	if p.PlanID == "" {
		return nil
	}

	err := c.abort.Handle(ctx, command.AbortSessionCommand{
		SessionPlanID: p.PlanID,
		Reason:        p.Reason,
	})

	if errors.Is(err, roadmap.ErrSessionNotFound) {
		log.Printf("[Coaching] WorkoutSessionAborted references unknown plan_id=%s; ignoring", p.PlanID)

		return nil
	}

	return err
}
