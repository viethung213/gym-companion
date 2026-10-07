package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestWorkoutMetrics(t *testing.T) {
	t.Run("valid positive metrics", func(t *testing.T) {
		m := vo.NewWorkoutMetrics("sess-1", "Chest Day", 3600, 5200.5, 5, 20, 2)

		if m.SessionID() != "sess-1" {
			t.Fatalf("got session id %q, want %q", m.SessionID(), "sess-1")
		}
		if m.WorkoutTitle() != "Chest Day" {
			t.Fatalf("got workout title %q, want %q", m.WorkoutTitle(), "Chest Day")
		}
		if m.DurationSeconds() != 3600 {
			t.Fatalf("got duration %d, want 3600", m.DurationSeconds())
		}
		if m.TotalVolumeKg() != 5200.5 {
			t.Fatalf("got volume %f, want 5200.5", m.TotalVolumeKg())
		}
		if m.ExerciseCount() != 5 {
			t.Fatalf("got exercise count %d, want 5", m.ExerciseCount())
		}
		if m.TotalSets() != 20 {
			t.Fatalf("got total sets %d, want 20", m.TotalSets())
		}
		if m.PRCount() != 2 {
			t.Fatalf("got pr count %d, want 2", m.PRCount())
		}
		if m.IsZero() {
			t.Fatalf("expected non-zero metrics")
		}
	})

	t.Run("negative values clamped to zero", func(t *testing.T) {
		m := vo.NewWorkoutMetrics("", "", -100, -50.0, -2, -5, -1)

		if m.DurationSeconds() != 0 {
			t.Fatalf("got duration %d, want 0", m.DurationSeconds())
		}
		if m.TotalVolumeKg() != 0 {
			t.Fatalf("got volume %f, want 0", m.TotalVolumeKg())
		}
		if m.ExerciseCount() != 0 {
			t.Fatalf("got exercise count %d, want 0", m.ExerciseCount())
		}
		if m.TotalSets() != 0 {
			t.Fatalf("got total sets %d, want 0", m.TotalSets())
		}
		if m.PRCount() != 0 {
			t.Fatalf("got pr count %d, want 0", m.PRCount())
		}
		if !m.IsZero() {
			t.Fatalf("expected IsZero to be true")
		}
	})
}
