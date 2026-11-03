package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/event"
)

type mockOutboxRepo struct {
	records []*port.OutboxRecord
	saveErr error
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepo) SaveEvent(ctx context.Context, eventID, eventType, partitionKey string, payload interface{}) error {
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockOutboxRepo) ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []port.OutboxRecord) error) error {
	return nil
}

func TestOutboxWriter_RecordEvents(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("successfully records multiple nutrition domain events", func(t *testing.T) {
		repo := &mockOutboxRepo{}
		writer := infraEvent.NewOutboxWriter(repo)

		ev1 := domainEvent.NewNutritionPlanGeneratedEvent("plan-1", "user-1", now, now)
		ev2 := domainEvent.NewMealLoggedEvent("meal-1", "user-1", "Breakfast", "BREAKFAST", 500, now)
		ev3 := domainEvent.NewLockoutAppliedEvent("user-1", "SUGAR", "Lockout applied", now.Add(24*time.Hour))

		err := writer.RecordEvents(ctx, []any{ev1, ev2, ev3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(repo.records) != 3 {
			t.Fatalf("expected 3 outbox records, got %d", len(repo.records))
		}

		// Verify record 1: NutritionPlanGenerated
		r1 := repo.records[0]
		if r1.AggregateType != "NutritionPlan" || r1.AggregateID != "plan-1" || r1.PartitionKey != "user-1" {
			t.Errorf("r1 routing mismatch: aggType=%s, aggID=%s, partKey=%s", r1.AggregateType, r1.AggregateID, r1.PartitionKey)
		}
		if r1.EventType != "contracts.core.nutrition.v1.event.NutritionPlanGenerated" {
			t.Errorf("r1 eventType mismatch: %s", r1.EventType)
		}

		// Verify record 2: MealLogged
		r2 := repo.records[1]
		if r2.AggregateType != "MealLog" || r2.AggregateID != "meal-1" || r2.PartitionKey != "user-1" {
			t.Errorf("r2 routing mismatch: aggType=%s, aggID=%s, partKey=%s", r2.AggregateType, r2.AggregateID, r2.PartitionKey)
		}

		// Verify record 3: LockoutApplied
		r3 := repo.records[2]
		if r3.AggregateType != "Lockout" || r3.AggregateID != "user-1" || r3.PartitionKey != "user-1" {
			t.Errorf("r3 routing mismatch: aggType=%s, aggID=%s, partKey=%s", r3.AggregateType, r3.AggregateID, r3.PartitionKey)
		}

		// Verify CloudEvent envelope format
		for idx, r := range repo.records {
			var env infraEvent.CloudEventEnvelope
			if err := json.Unmarshal(r.Payload, &env); err != nil {
				t.Fatalf("record %d payload is not valid CloudEvent envelope: %v", idx, err)
			}
			if env.SpecVersion != "1.0" {
				t.Errorf("record %d: got specversion %s, want 1.0", idx, env.SpecVersion)
			}
			if env.Source != "services/nutrition-service" {
				t.Errorf("record %d: got source %s, want services/nutrition-service", idx, env.Source)
			}
			if env.DataContentType != "application/json" {
				t.Errorf("record %d: got datacontenttype %s, want application/json", idx, env.DataContentType)
			}
		}
	})

	t.Run("returns error when unsupported event type provided", func(t *testing.T) {
		repo := &mockOutboxRepo{}
		writer := infraEvent.NewOutboxWriter(repo)

		err := writer.RecordEvents(ctx, []any{"invalid_string_event"})
		if err == nil {
			t.Fatal("expected error for unsupported event type, got nil")
		}
	})

	t.Run("returns error when outbox repo Save fails", func(t *testing.T) {
		expectedErr := errors.New("db connection failure")
		repo := &mockOutboxRepo{saveErr: expectedErr}
		writer := infraEvent.NewOutboxWriter(repo)

		ev := domainEvent.NewNutritionPlanGeneratedEvent("plan-1", "user-1", now, now)
		err := writer.RecordEvents(ctx, []any{ev})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("PublishEvents delegates to RecordEvents", func(t *testing.T) {
		repo := &mockOutboxRepo{}
		writer := infraEvent.NewOutboxWriter(repo)

		ev := domainEvent.NewNutritionPlanRecalibratedEvent("plan-1", "user-1", "Metabolic shift", now)
		err := writer.PublishEvents(ctx, []any{ev})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(repo.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(repo.records))
		}
		if repo.records[0].EventType != "contracts.core.nutrition.v1.event.NutritionPlanRecalibrated" {
			t.Errorf("got eventType %s, want NutritionPlanRecalibrated", repo.records[0].EventType)
		}
	})
}
