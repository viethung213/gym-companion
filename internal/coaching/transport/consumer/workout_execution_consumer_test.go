package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type stubOutbox struct {
	seen map[string]bool
}

func (s *stubOutbox) Save(context.Context, *port.OutboxRecord) error { return nil }
func (s *stubOutbox) FetchUnpublished(context.Context, int) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (s *stubOutbox) ClaimBatch(context.Context, int, time.Duration) ([]*port.OutboxRecord, error) {
	return nil, nil
}
func (s *stubOutbox) MarkPublished(context.Context, []string) error { return nil }
func (s *stubOutbox) ProcessBatch(ctx context.Context, _ int, publishFn func(context.Context, []*port.OutboxRecord) error) error {
	return publishFn(ctx, nil)
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
		"id":          id,
		"source":      "test",
		"type":        typeStr,
		"data":        json.RawMessage(dataBytes),
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}

	return b
}

func TestConsumer_UnknownEventType_Ignored(t *testing.T) {
	c := NewWorkoutExecutionConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeCE(t, "evt-1", "some.unknown.event", map[string]string{"x": "y"})

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("expected nil for unknown type, got %v", err)
	}
}

func TestConsumer_MissingEventID(t *testing.T) {
	c := NewWorkoutExecutionConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeCE(t, "", "contracts.core.workout_execution.v1.workoutSessionCompleted",
		map[string]any{"planId": "sp-1"})

	if err := c.HandleMessage(context.Background(), raw); err == nil {
		t.Errorf("expected error for missing event id")
	}
}

func TestConsumer_DuplicateEvent_Skipped(t *testing.T) {
	stub := &stubOutbox{}
	c := NewWorkoutExecutionConsumer(nil, nil, nil, stub)

	raw := makeCE(t, "evt-1", "contracts.core.workout_execution.v1.workoutSessionCompleted",
		map[string]any{"planId": "" /* empty planId to short-circuit before handler */})

	// First call: fresh — since PlanID empty, handler no-ops.
	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second call: duplicate — outbox_log dedup returns fresh=false.
	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("second (duplicate): %v", err)
	}
}

func TestWorkoutSessionCompletedPayload_Unmarshal(t *testing.T) {
	t.Run("camelCase protojson mapping", func(t *testing.T) {
		raw := []byte(`{
			"sessionId": "sess-1",
			"userId": "user-1",
			"completedAt": "2026-08-09T00:00:00Z",
			"totalSets": 12,
			"totalVolume": 1500.5,
			"averageFormScore": 92.5,
			"averageRpe": 8.0,
			"planId": "plan-101"
		}`)

		var p WorkoutSessionCompletedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-101"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.SessionID, "sess-1"; got != want {
			t.Errorf("SessionID got %s, want %s", got, want)
		}
		if got, want := p.UserID, "user-1"; got != want {
			t.Errorf("UserID got %s, want %s", got, want)
		}
		if got, want := p.TotalSets, int32(12); got != want {
			t.Errorf("TotalSets got %d, want %d", got, want)
		}
		if got, want := p.TotalVolume, float32(1500.5); got != want {
			t.Errorf("TotalVolume got %f, want %f", got, want)
		}
		if got, want := p.AverageFormScore, float32(92.5); got != want {
			t.Errorf("AverageFormScore got %f, want %f", got, want)
		}
		if got, want := p.AverageRPE, float32(8.0); got != want {
			t.Errorf("AverageRPE got %f, want %f", got, want)
		}
	})

	t.Run("snake_case fallback mapping", func(t *testing.T) {
		raw := []byte(`{
			"session_id": "sess-2",
			"user_id": "user-2",
			"completed_at": "2026-08-09T00:00:00Z",
			"total_sets": 8,
			"total_volume": 800.0,
			"average_form_score": 85.0,
			"average_rpe": 7.5,
			"plan_id": "plan-202"
		}`)

		var p WorkoutSessionCompletedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-202"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.SessionID, "sess-2"; got != want {
			t.Errorf("SessionID got %s, want %s", got, want)
		}
		if got, want := p.AverageRPE, float32(7.5); got != want {
			t.Errorf("AverageRPE got %f, want %f", got, want)
		}
	})
}

func TestWorkoutSessionAbortedPayload_Unmarshal(t *testing.T) {
	t.Run("camelCase mapping", func(t *testing.T) {
		raw := []byte(`{
			"sessionId": "sess-3",
			"userId": "user-3",
			"abortedAt": "2026-08-09T00:00:00Z",
			"reason": "timeout",
			"planId": "plan-303"
		}`)

		var p WorkoutSessionAbortedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-303"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.Reason, "timeout"; got != want {
			t.Errorf("Reason got %s, want %s", got, want)
		}
	})

	t.Run("snake_case mapping", func(t *testing.T) {
		raw := []byte(`{
			"session_id": "sess-4",
			"user_id": "user-4",
			"aborted_at": "2026-08-09T00:00:00Z",
			"reason": "user_cancelled",
			"plan_id": "plan-404"
		}`)

		var p WorkoutSessionAbortedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-404"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.Reason, "user_cancelled"; got != want {
			t.Errorf("Reason got %s, want %s", got, want)
		}
	})
}
