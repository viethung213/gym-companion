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
	if got, want := ev1.EventName(), "contracts.core.workout_execution.v1.event.WorkoutSessionStarted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev2 := event.WorkoutSessionCompleted{SessionID: "s1", UserID: "u1", PlanID: "p1", CompletedAt: now, Summary: vo.SessionSummary{}}
	if got, want := ev2.EventName(), "contracts.core.workout_execution.v1.event.WorkoutSessionCompleted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev3 := event.WorkoutSessionAborted{SessionID: "s1", UserID: "u1", PlanID: "p1", Reason: "stop", IsAnomalous: false, AbortedAt: now}
	if got, want := ev3.EventName(), "contracts.core.workout_execution.v1.event.WorkoutSessionAborted"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev4 := event.NewPersonalRecordAchieved{UserID: "u1", ExerciseID: "ex1", OneRepMax: 100, Weight: 100, Reps: 10, FormVerified: true, AchievedAt: now}
	if got, want := ev4.EventName(), "contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ev5 := event.BodyMetricUpdated{UserID: "u1", WeightKg: 75.0, RecordedAt: now}
	if got, want := ev5.EventName(), "contracts.core.workout_execution.v1.event.BodyMetricUpdated"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
