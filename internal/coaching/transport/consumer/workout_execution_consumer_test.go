package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type stubOutbox struct {
	seen map[string]bool
}

func (s *stubOutbox) Save(context.Context, *port.OutboxRecord) error { return nil }

func (s *stubOutbox) FetchUnpublished(context.Context, int) ([]*port.OutboxRecord, error) {

	return nil, nil

}

func (s *stubOutbox) MarkPublished(context.Context, []string) error { return nil }

func (s *stubOutbox) ExecuteInLock(ctx context.Context, _ int64, fn func(context.Context) error) error {

	return fn(ctx)

}

func (s *stubOutbox) LogProcessed(_ context.Context, eventID, _, _ string, _ []byte) (bool, error) {

	if s.seen == nil {

		s.seen = map[string]bool{}

	}

	if s.seen[eventID] {

		return false, nil

	}

	s.seen[eventID] = true

	return true, nil

}

func makeCE(t *testing.T, id, typeStr string, dataObj any) []byte {

	t.Helper()

	dataBytes, err := json.Marshal(dataObj)

	if err != nil {

		t.Fatalf("marshal data: %v", err)

	}

	env := map[string]any{

		"specversion": "1.0",

		"id": id,

		"source": "test",

		"type": typeStr,

		"data": json.RawMessage(dataBytes),
	}

	b, err := json.Marshal(env)

	if err != nil {

		t.Fatalf("marshal env: %v", err)

	}

	return b

}

func TestConsumer_UnknownEventType_Ignored(t *testing.T) {

	c := NewWorkoutExecutionConsumer(nil, nil, &stubOutbox{})

	raw := makeCE(t, "evt-1", "some.unknown.event", map[string]string{"x": "y"})

	if err := c.HandleMessage(context.Background(), raw); err != nil {

		t.Errorf("expected nil for unknown type, got %v", err)

	}

}

func TestConsumer_MissingEventID(t *testing.T) {

	c := NewWorkoutExecutionConsumer(nil, nil, &stubOutbox{})

	raw := makeCE(t, "", "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted",

		WorkoutSessionCompletedPayload{PlanID: "sp-1"})

	if err := c.HandleMessage(context.Background(), raw); err == nil {

		t.Errorf("expected error for missing event id")

	}

}

func TestConsumer_DuplicateEvent_Skipped(t *testing.T) {

	stub := &stubOutbox{}

	c := NewWorkoutExecutionConsumer(nil, nil, stub)

	raw := makeCE(t, "evt-1", "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted",

		WorkoutSessionCompletedPayload{PlanID: "" /* deliberately empty to short-circuit before handler */})

	// First call: fresh — since PlanID empty, handler no-ops.

	if err := c.HandleMessage(context.Background(), raw); err != nil {

		t.Fatalf("first: %v", err)

	}

	// Second call: duplicate — outbox_log dedup returns fresh=false.

	if err := c.HandleMessage(context.Background(), raw); err != nil {

		t.Errorf("second (duplicate): %v", err)

	}

}
