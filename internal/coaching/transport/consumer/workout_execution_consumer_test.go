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

func TestSoloTrackingCompletedPayload_Unmarshal(t *testing.T) {
	t.Run("camelCase protojson mapping", func(t *testing.T) {
		raw := []byte(`{
			"matchId": "match-101",
			"userId": "user-101",
			"exerciseId": "exercise_squat_01",
			"challengeType": "REPS",
			"confirmedValue": 25,
			"averageFormScore": 88.5,
			"totalDurationSeconds": 180,
			"completedAt": "2026-10-22T08:00:00Z",
			"planId": "plan-solo-999"
		}`)

		var p SoloTrackingCompletedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-solo-999"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.MatchID, "match-101"; got != want {
			t.Errorf("MatchID got %s, want %s", got, want)
		}
		if got, want := p.UserID, "user-101"; got != want {
			t.Errorf("UserID got %s, want %s", got, want)
		}
		if got, want := p.ExerciseID, "exercise_squat_01"; got != want {
			t.Errorf("ExerciseID got %s, want %s", got, want)
		}
		if got, want := p.ChallengeType, "REPS"; got != want {
			t.Errorf("ChallengeType got %s, want %s", got, want)
		}
		if got, want := p.ConfirmedValue, int32(25); got != want {
			t.Errorf("ConfirmedValue got %d, want %d", got, want)
		}
		if got, want := p.AverageFormScore, float32(88.5); got != want {
			t.Errorf("AverageFormScore got %f, want %f", got, want)
		}
		if got, want := p.TotalDurationSeconds, int32(180); got != want {
			t.Errorf("TotalDurationSeconds got %d, want %d", got, want)
		}
		if got, want := p.CompletedAt, "2026-10-22T08:00:00Z"; got != want {
			t.Errorf("CompletedAt got %s, want %s", got, want)
		}
	})

	t.Run("snake_case fallback mapping", func(t *testing.T) {
		raw := []byte(`{
			"match_id": "match-202",
			"user_id": "user-202",
			"exercise_id": "exercise_plank_01",
			"challenge_type": "DURATION",
			"confirmed_value": 45,
			"average_form_score": 91.0,
			"total_duration_seconds": 45,
			"completed_at": "2026-10-22T08:05:00Z",
			"plan_id": "plan-solo-888"
		}`)

		var p SoloTrackingCompletedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-solo-888"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.MatchID, "match-202"; got != want {
			t.Errorf("MatchID got %s, want %s", got, want)
		}
		if got, want := p.ConfirmedValue, int32(45); got != want {
			t.Errorf("ConfirmedValue got %d, want %d", got, want)
		}
		if got, want := p.AverageFormScore, float32(91.0); got != want {
			t.Errorf("AverageFormScore got %f, want %f", got, want)
		}
	})
}

func TestConsumer_SoloTrackingCompleted_EmptyPlanId_Ignored(t *testing.T) {
	c := NewWorkoutExecutionConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeCE(t, "evt-solo-1", "contracts.core.workout_execution.v1.soloTrackingCompleted",
		map[string]any{"planId": "", "matchId": "match-101"})

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("expected nil when planId is empty, got %v", err)
	}
}

func TestSoloTrackingAbortedPayload_Unmarshal(t *testing.T) {
	t.Run("camelCase protojson mapping", func(t *testing.T) {
		raw := []byte(`{
			"matchId": "match-abort-101",
			"userId": "user-abort-101",
			"exerciseId": "exercise_squat_01",
			"reason": "USER_FORFEITED",
			"abortedAt": "2026-10-22T08:10:00Z",
			"planId": "plan-abort-999",
			"confirmedValue": 12,
			"totalDurationSeconds": 45
		}`)

		var p SoloTrackingAbortedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-abort-999"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.MatchID, "match-abort-101"; got != want {
			t.Errorf("MatchID got %s, want %s", got, want)
		}
		if got, want := p.Reason, "USER_FORFEITED"; got != want {
			t.Errorf("Reason got %s, want %s", got, want)
		}
		if got, want := p.ConfirmedValue, int32(12); got != want {
			t.Errorf("ConfirmedValue got %d, want %d", got, want)
		}
	})

	t.Run("snake_case fallback mapping", func(t *testing.T) {
		raw := []byte(`{
			"match_id": "match-abort-202",
			"user_id": "user-abort-202",
			"exercise_id": "exercise_plank_01",
			"reason": "USER_DISCONNECTED",
			"aborted_at": "2026-10-22T08:15:00Z",
			"plan_id": "plan-abort-888",
			"confirmed_value": 20,
			"total_duration_seconds": 25
		}`)

		var p SoloTrackingAbortedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if got, want := p.PlanID, "plan-abort-888"; got != want {
			t.Errorf("PlanID got %s, want %s", got, want)
		}
		if got, want := p.Reason, "USER_DISCONNECTED"; got != want {
			t.Errorf("Reason got %s, want %s", got, want)
		}
	})
}

func TestConsumer_SoloTrackingAborted_EmptyPlanId_Ignored(t *testing.T) {
	c := NewWorkoutExecutionConsumer(nil, nil, nil, &stubOutbox{})

	raw := makeCE(t, "evt-solo-abort-1", "contracts.core.workout_execution.v1.soloTrackingAborted",
		map[string]any{"planId": "", "matchId": "match-abort-101"})

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Errorf("expected nil when planId is empty, got %v", err)
	}
}
