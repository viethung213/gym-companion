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
	"time"

	segmentio "github.com/segmentio/kafka-go"
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
// It flexibly decodes both camelCase (protojson) and snake_case JSON keys.
type WorkoutSessionCompletedPayload struct {
	SessionID        string  `json:"sessionId"`
	UserID           string  `json:"userId"`
	CompletedAt      string  `json:"completedAt"` // RFC3339
	TotalSets        int32   `json:"totalSets"`
	TotalVolume      float32 `json:"totalVolume"`
	AverageFormScore float32 `json:"averageFormScore"`
	AverageRPE       float32 `json:"averageRpe"`
	PlanID           string  `json:"planId"`
}

// UnmarshalJSON implements custom unmarshaling to support both camelCase and snake_case.
func (p *WorkoutSessionCompletedPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		// camelCase (protojson)
		SessionIDCamel        string   `json:"sessionId"`
		UserIDCamel           string   `json:"userId"`
		CompletedAtCamel      string   `json:"completedAt"`
		TotalSetsCamel        int32    `json:"totalSets"`
		TotalVolumeCamel      float32  `json:"totalVolume"`
		AverageFormScoreCamel *float32 `json:"averageFormScore"`
		AverageRPECamel       float32  `json:"averageRpe"`
		PlanIDCamel           string   `json:"planId"`

		// snake_case
		SessionIDSnake        string   `json:"session_id"`
		UserIDSnake           string   `json:"user_id"`
		CompletedAtSnake      string   `json:"completed_at"`
		TotalSetsSnake        int32    `json:"total_sets"`
		TotalVolumeSnake      float32  `json:"total_volume"`
		AverageFormScoreSnake *float32 `json:"average_form_score"`
		AverageRPESnake       float32  `json:"average_rpe"`
		PlanIDSnake           string   `json:"plan_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.PlanIDCamel != "" {
		p.PlanID = raw.PlanIDCamel
	} else {
		p.PlanID = raw.PlanIDSnake
	}

	if raw.SessionIDCamel != "" {
		p.SessionID = raw.SessionIDCamel
	} else {
		p.SessionID = raw.SessionIDSnake
	}

	if raw.UserIDCamel != "" {
		p.UserID = raw.UserIDCamel
	} else {
		p.UserID = raw.UserIDSnake
	}

	if raw.CompletedAtCamel != "" {
		p.CompletedAt = raw.CompletedAtCamel
	} else {
		p.CompletedAt = raw.CompletedAtSnake
	}

	if raw.TotalSetsCamel != 0 {
		p.TotalSets = raw.TotalSetsCamel
	} else {
		p.TotalSets = raw.TotalSetsSnake
	}

	if raw.TotalVolumeCamel != 0 {
		p.TotalVolume = raw.TotalVolumeCamel
	} else {
		p.TotalVolume = raw.TotalVolumeSnake
	}

	if raw.AverageFormScoreCamel != nil {
		p.AverageFormScore = *raw.AverageFormScoreCamel
	} else if raw.AverageFormScoreSnake != nil {
		p.AverageFormScore = *raw.AverageFormScoreSnake
	}

	if raw.AverageRPECamel != 0 {
		p.AverageRPE = raw.AverageRPECamel
	} else {
		p.AverageRPE = raw.AverageRPESnake
	}

	return nil
}

// WorkoutSessionAbortedPayload mirrors the proto CloudEvent data field.
// It flexibly decodes both camelCase (protojson) and snake_case JSON keys.
type WorkoutSessionAbortedPayload struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	AbortedAt string `json:"abortedAt"`
	Reason    string `json:"reason"`
	PlanID    string `json:"planId"`
}

func (p *WorkoutSessionAbortedPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		// camelCase (protojson)
		SessionIDCamel string `json:"sessionId"`
		UserIDCamel    string `json:"userId"`
		AbortedAtCamel string `json:"abortedAt"`
		PlanIDCamel    string `json:"planId"`

		// snake_case
		SessionIDSnake string `json:"session_id"`
		UserIDSnake    string `json:"user_id"`
		AbortedAtSnake string `json:"aborted_at"`
		PlanIDSnake    string `json:"plan_id"`

		// Shared field
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Reason = raw.Reason

	if raw.PlanIDCamel != "" {
		p.PlanID = raw.PlanIDCamel
	} else {
		p.PlanID = raw.PlanIDSnake
	}

	if raw.SessionIDCamel != "" {
		p.SessionID = raw.SessionIDCamel
	} else {
		p.SessionID = raw.SessionIDSnake
	}

	if raw.UserIDCamel != "" {
		p.UserID = raw.UserIDCamel
	} else {
		p.UserID = raw.UserIDSnake
	}

	if raw.AbortedAtCamel != "" {
		p.AbortedAt = raw.AbortedAtCamel
	} else {
		p.AbortedAt = raw.AbortedAtSnake
	}

	return nil
}

// WorkoutExecutionConsumer routes WorkoutSessionCompleted/Aborted events to
// the appropriate command handler.
type WorkoutExecutionConsumer struct {
	reader   *segmentio.Reader
	complete *command.CompleteSessionHandler
	abort    *command.AbortSessionHandler
	outbox   port.OutboxRepository
}

// NewWorkoutExecutionConsumer wires the consumer.
func NewWorkoutExecutionConsumer(
	reader *segmentio.Reader,
	complete *command.CompleteSessionHandler,
	abort *command.AbortSessionHandler,
	outbox port.OutboxRepository,
) *WorkoutExecutionConsumer {
	return &WorkoutExecutionConsumer{
		reader:   reader,
		complete: complete,
		abort:    abort,
		outbox:   outbox,
	}
}

// Start begins the blocking read loop. It stops when ctx is cancelled.
func (c *WorkoutExecutionConsumer) Start(ctx context.Context) {
	log.Println("[Coaching] Starting WorkoutExecutionConsumer on topic workout_execution.events...")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Coaching] Stopping WorkoutExecutionConsumer due to context cancellation.")
			return
		default:
			if c.reader == nil {
				log.Println("[Coaching] WorkoutExecutionConsumer reader is nil; stopping loop.")
				return
			}

			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				log.Printf("[Coaching] Error reading workout_execution.events: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if processErr := c.processWithRetry(ctx, &msg); processErr != nil {
				log.Printf("[Coaching] Exhausted retries processing workout execution event: %v", processErr)
			}
		}
	}
}

// processWithRetry attempts to handle the message up to maxRetries times with
// incremental backoff (100ms per attempt number).
func (c *WorkoutExecutionConsumer) processWithRetry(ctx context.Context, msg *segmentio.Message) error {
	const maxRetries = 3

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = c.HandleMessage(ctx, msg.Value)
		if lastErr == nil {
			return nil
		}

		log.Printf("[Coaching] Workout event attempt %d/%d failed: %v", attempt, maxRetries, lastErr)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
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
	case "contracts.core.workout_execution.v1.workoutSessionCompleted",
		"contracts.core.workout_execution.v1.event.WorkoutSessionCompleted":
		return c.handleCompleted(ctx, env.Data)

	case "contracts.core.workout_execution.v1.workoutSessionAborted",
		"contracts.core.workout_execution.v1.event.WorkoutSessionAborted":
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
		SessionPlanID:       p.PlanID,
		TotalActualSets:     int(p.TotalSets),
		TotalPrescribedSets: 0, // Best-effort: recomputed inside handler if available.
		AverageActualRPE:    float64(p.AverageRPE),
		CompletedAt:         p.CompletedAt,
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
