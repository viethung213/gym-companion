// Package event holds the outbox writer that wraps coaching domain events
// in CloudEvents envelopes and persists them via the outbox repository.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	coachingv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/event"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Source is the CloudEvents `source` field for all coaching events.
const Source = "services/coaching-service"

// OutboxWriter implements port.OutboxWriter.
type OutboxWriter struct {
	outbox port.OutboxRepository
}

var _ port.OutboxWriter = (*OutboxWriter)(nil)

// NewOutboxWriter constructs the writer.
func NewOutboxWriter(outbox port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outbox: outbox}
}

// Enqueue serializes each domain event into a CloudEvents envelope and stores
// it in coaching.outbox with the given partition key. All writes should be
// inside the caller's transaction.
func (w *OutboxWriter) Enqueue(ctx context.Context, partitionKey string, events ...domainevent.Event) error {
	for _, ev := range events {
		if err := w.enqueueOne(ctx, partitionKey, ev); err != nil {
			return err
		}
	}

	return nil
}

func (w *OutboxWriter) enqueueOne(ctx context.Context, partitionKey string, ev domainevent.Event) error {
	payloadBytes, err := w.marshalPayload(ev)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	eventID := uuid.NewString()
	now := time.Now().UTC()

	envelope := map[string]any{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          Source,
		"type":            ev.EventName(),
		"time":            now.Format(time.RFC3339Nano),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	rec := &port.OutboxRecord{
		ID:           uuid.NewString(),
		EventID:      eventID,
		EventType:    ev.EventName(),
		Payload:      envBytes,
		PartitionKey: partitionKey,
		CreatedAt:    now,
		Published:    false,
	}

	return w.outbox.Save(ctx, rec)
}

func (w *OutboxWriter) marshalPayload(ev domainevent.Event) ([]byte, error) {
	switch e := ev.(type) {
	case *domainevent.UpcomingWorkoutReminder:
		protoEv := &coachingv1event.UpcomingWorkoutReminder{
			UserId:              e.UserID,
			WorkoutName:         e.WorkoutName,
			ScheduledTime:       e.ScheduledTime,
			RemindBeforeMinutes: e.RemindBeforeMinutes,
		}
		return protojson.Marshal(protoEv)

	case *domainevent.RoadmapInitiated:
		protoEv := &coachingv1event.RoadmapInitiated{
			RoadmapId:   e.RoadmapID,
			UserId:      e.UserID,
			InitiatedAt: timestamppb.New(e.InitiatedAt),
		}
		return protojson.Marshal(protoEv)

	case *domainevent.RoadmapAdjusted:
		protoEv := &coachingv1event.RoadmapAdjusted{
			RoadmapId:  e.RoadmapID,
			UserId:     e.UserID,
			Reason:     e.Reason,
			AdjustedAt: timestamppb.New(e.AdjustedAt),
		}
		return protojson.Marshal(protoEv)

	case *domainevent.SessionPlanExecuted:
		protoEv := &coachingv1event.SessionPlanExecuted{
			SessionPlanId:   e.SessionPlanID,
			RoadmapId:       e.RoadmapID,
			UserId:          e.UserID,
			ExecutedAt:      timestamppb.New(e.ExecutedAt),
			SessionScr:      e.SessionSCR,
			SessionDeltaRpe: e.SessionDeltaRPE,
		}
		return protojson.Marshal(protoEv)

	default:
		return json.Marshal(ev)
	}
}
