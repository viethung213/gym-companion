package event_test

import (
	"encoding/json"
	"testing"
	"time"

	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/event"
)

func TestPackCloudEvent_MealLogged(t *testing.T) {
	now := time.Now().UTC()
	ev := domainEvent.NewMealLoggedEvent("meal-1", "user-1", "Lunch", "LUNCH", 650.0, now)

	rec, err := infraEvent.PackCloudEvent(ev)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if rec.EventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if rec.EventType != "contracts.core.nutrition.v1.event.MealLogged" {
		t.Errorf("got eventType %s, want MealLogged", rec.EventType)
	}
	if rec.AggregateType != "MealLog" || rec.AggregateID != "meal-1" || rec.PartitionKey != "user-1" {
		t.Errorf("unexpected routing metadata: %s, %s, %s", rec.AggregateType, rec.AggregateID, rec.PartitionKey)
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
	if envelope.Source != "services/nutrition-service" {
		t.Errorf("got source %s, want services/nutrition-service", envelope.Source)
	}
}
