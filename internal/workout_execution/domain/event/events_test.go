package event_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestDomainEventNames(t *testing.T) {
	now := time.Now().UTC()

	ev1 := event.WorkoutSessionStarted{SessionID: "s1", UserID: "u1", PlanID: "p1", StartedAt: now}
	if got, want := ev1.EventName(), "contracts.core.workout_execution.v1.workoutSessionStarted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev2 := event.WorkoutSessionCompleted{SessionID: "s1", UserID: "u1", CompletedAt: now, Summary: vo.SessionSummary{}}
	if got, want := ev2.EventName(), "contracts.core.workout_execution.v1.workoutSessionCompleted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev3 := event.WorkoutSessionAborted{SessionID: "s1", UserID: "u1", Reason: "stop", IsAnomalous: false, AbortedAt: now}
	if got, want := ev3.EventName(), "contracts.core.workout_execution.v1.workoutSessionAborted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev4 := event.NewPersonalRecordAchieved{UserID: "u1", ExerciseID: "ex1", OneRepMax: 100, Weight: 100, Reps: 10, FormVerified: true, AchievedAt: now}
	if got, want := ev4.EventName(), "contracts.core.workout_execution.v1.newPersonalRecordAchieved"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev5 := event.BodyMetricUpdated{UserID: "u1", WeightKg: 75.0, RecordedAt: now}
	if got, want := ev5.EventName(), "contracts.core.workout_execution.v1.bodyMetricUpdated"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
