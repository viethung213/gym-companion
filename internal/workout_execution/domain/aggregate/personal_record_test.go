package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
)

func TestPersonalRecord_Calculate1RMEpley(t *testing.T) {
	tests := []struct {
		name   string
		weight float32
		reps   int
		want   float32
	}{
		{name: "zero reps", weight: 100, reps: 0, want: 0},
		{name: "zero weight", weight: 0, reps: 10, want: 0},
		{name: "one rep returns exact weight", weight: 100, reps: 1, want: 100},
		{name: "multiple reps epley formula", weight: 100, reps: 10, want: 133.33333},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregate.Calculate1RMEpley(tt.weight, tt.reps)
			if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
				t.Errorf("got Calculate1RMEpley = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersonalRecord_Lifecycle(t *testing.T) {
	t.Run("NewPersonalRecord", func(t *testing.T) {
		pr := aggregate.NewPersonalRecord("pr-1", "user-1", "ex-1", 100.0, 10, true, time.Time{})
		if pr.ID() != "pr-1" || pr.UserID() != "user-1" || pr.ExerciseID() != "ex-1" {
			t.Errorf("invalid PR getters")
		}
		if pr.Weight() != 100.0 || pr.Reps() != 10 || !pr.FormVerified() {
			t.Errorf("invalid PR attributes")
		}
		if pr.OneRepMax() <= 0 {
			t.Errorf("got OneRepMax = %v, want > 0", pr.OneRepMax())
		}
		events := pr.PopEvents()
		if len(events) != 1 {
			t.Errorf("got events count = %d, want 1", len(events))
		}
	})

	t.Run("UpdateIfHigher strictly higher", func(t *testing.T) {
		pr := aggregate.NewPersonalRecord("pr-1", "user-1", "ex-1", 80.0, 10, true, time.Now().UTC())
		_ = pr.PopEvents()

		// Higher performance: 100kg x 10 reps
		updated := pr.UpdateIfHigher(100.0, 10, true, time.Time{})
		if !updated {
			t.Errorf("got updated = false, want true")
		}
		if pr.Weight() != 100.0 {
			t.Errorf("got weight = %v, want 100.0", pr.Weight())
		}
		if len(pr.PopEvents()) != 1 {
			t.Errorf("want 1 event published on PR update")
		}

		// Lower/equal performance: 50kg x 5 reps -> Should not update
		notUpdated := pr.UpdateIfHigher(50.0, 5, true, time.Now().UTC())
		if notUpdated {
			t.Errorf("got updated = true, want false for lower performance")
		}
	})
}
