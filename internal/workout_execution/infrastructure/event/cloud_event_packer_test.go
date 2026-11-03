package event_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
)

func TestPackCloudEvent_WorkoutSessionStarted(t *testing.T) {
	now := time.Now().UTC()
	ev := &event.WorkoutSessionStarted{
		SessionID: "sess-1",
		UserID:    "user-1",
		PlanID:    "plan-1",
		StartedAt: now,
	}

	rec, err := infraEvent.PackCloudEvent(ev)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if rec.EventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if rec.EventType != "contracts.core.workout_execution.v1.workoutSessionStarted" {
		t.Errorf("got eventType %s, want contracts.core.workout_execution.v1.workoutSessionStarted", rec.EventType)
	}
	if rec.AggregateType != "WorkoutSession" {
		t.Errorf("got aggType %s, want WorkoutSession", rec.AggregateType)
	}
	if rec.AggregateID != "sess-1" {
		t.Errorf("got aggID %s, want sess-1", rec.AggregateID)
	}
	if rec.PartitionKey != "user-1" {
		t.Errorf("got partitionKey %s, want user-1", rec.PartitionKey)
	}
	if !rec.OccurredAt.Equal(now) {
		t.Errorf("got occurredAt %v, want %v", rec.OccurredAt, now)
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if envelope.SpecVersion != "1.0" {
		t.Errorf("got specversion %s, want 1.0", envelope.SpecVersion)
	}
	if envelope.Source != "services/workout-execution-service" {
		t.Errorf("got source %s, want services/workout-execution-service", envelope.Source)
	}
}
