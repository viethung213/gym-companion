package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
)

type mockOutboxRepo struct {
	saveErr error

	savedRecord *port.OutboxRecord
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {

	m.savedRecord = record

	return m.saveErr

}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {

	return nil, nil

}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, ids []string) error {

	return nil

}

func (m *mockOutboxRepo) ExecuteInLock(ctx context.Context, lockID int64, fn func(txCtx context.Context) error) error {

	return nil

}

func TestOutboxWriter(t *testing.T) {

	t.Run("WriteEvents with valid domain event", func(t *testing.T) {

		repo := &mockOutboxRepo{}

		writer := infraEvent.NewOutboxWriter(repo)

		ev := event.WorkoutSessionStarted{

			SessionID: "sess-1",

			UserID: "user-1",

			PlanID: "plan-1",

			StartedAt: time.Now().UTC(),
		}

		err := writer.WriteEvents(context.Background(), "WorkoutSession", "sess-1", []interface{}{ev})

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

		if repo.savedRecord == nil {

			t.Fatal("savedRecord is nil")

		}

		if got, want := repo.savedRecord.EventType, "contracts.core.workout_execution.v1.workoutSessionStarted"; got != want {

			t.Errorf("got EventType = %v, want %v", got, want)

		}

		var envelope map[string]interface{}

		if err := json.Unmarshal(repo.savedRecord.Payload, &envelope); err != nil {

			t.Fatalf("failed to unmarshal payload: %v", err)

		}

		if got, want := envelope["type"], "contracts.core.workout_execution.v1.workoutSessionStarted"; got != want {

			t.Errorf("got envelope type = %v, want %v", got, want)

		}

		dataMap, ok := envelope["data"].(map[string]interface{})

		if !ok {

			t.Fatalf("expected data to be map[string]interface{}, got %T", envelope["data"])

		}

		if got, want := dataMap["sessionId"], "sess-1"; got != want {

			t.Errorf("got data.sessionId = %v, want %v", got, want)

		}

		if got, want := dataMap["userId"], "user-1"; got != want {

			t.Errorf("got data.userId = %v, want %v", got, want)

		}

	})

	t.Run("WriteEvents with generic struct (unknown event type)", func(t *testing.T) {

		repo := &mockOutboxRepo{}

		writer := infraEvent.NewOutboxWriter(repo)

		genericEv := struct {
			Foo string `json:"foo"`
		}{Foo: "bar"}

		err := writer.WriteEvents(context.Background(), "Aggregate", "id-1", []interface{}{genericEv})

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

	})

	t.Run("WriteEvents unmarshalable payload error", func(t *testing.T) {

		repo := &mockOutboxRepo{}

		writer := infraEvent.NewOutboxWriter(repo)

		unmarshalable := make(chan int)

		err := writer.WriteEvents(context.Background(), "Aggregate", "id-1", []interface{}{unmarshalable})

		if err == nil {

			t.Fatal("got nil, want marshal error")

		}

	})

	t.Run("WriteEvents repo save error", func(t *testing.T) {

		repo := &mockOutboxRepo{saveErr: errors.New("db error")}

		writer := infraEvent.NewOutboxWriter(repo)

		ev := event.WorkoutSessionStarted{SessionID: "sess-1"}

		err := writer.WriteEvents(context.Background(), "WorkoutSession", "sess-1", []interface{}{ev})

		if err == nil {

			t.Fatal("got nil, want error")

		}

	})

}
