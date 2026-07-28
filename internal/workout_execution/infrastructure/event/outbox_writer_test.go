package event_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
)

type mockOutboxRepo struct {
	saveErr error
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
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
			UserID:    "user-1",
			PlanID:    "plan-1",
			StartedAt: time.Now().UTC(),
		}

		err := writer.WriteEvents(context.Background(), "WorkoutSession", "sess-1", []interface{}{ev})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
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
