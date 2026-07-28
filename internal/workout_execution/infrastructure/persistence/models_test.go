package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
)

func TestModels_TableNames(t *testing.T) {
	if got, want := (persistence.WorkoutSessionModel{}).TableName(), "workout_execution.workout_sessions"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.WorkoutSetLogModel{}).TableName(), "workout_execution.workout_set_logs"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.RepLogModel{}).TableName(), "workout_execution.rep_logs"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.SessionErrorModel{}).TableName(), "workout_execution.session_errors"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.PersonalRecordModel{}).TableName(), "workout_execution.personal_records"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.MotionSpecificationModel{}).TableName(), "workout_execution.motion_specifications"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := (persistence.OutboxModel{}).TableName(), "workout_execution.outbox"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestModels_SessionToPersistenceAndDomain(t *testing.T) {
	session, err := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	formScore := float32(88.5)
	repLog := vo.RepLog{
		RepNumber:     1,
		ROMPercentage: 95.0,
		ErrorCodes:    []string{"ERR_1"},
		JointAngles:   map[string]float32{"knee": 110.0},
	}

	_ = session.LogSet(aggregate.WorkoutSetLog{
		ID:          "set-1",
		SetNumber:   1,
		ExerciseID:  "ex-1",
		TargetReps:  10,
		ActualReps:  10,
		Weight:      60.0,
		FormScore:   &formScore,
		RPE:         8.0,
		CameraAngle: "front",
		Reps:        []vo.RepLog{repLog},
		CreatedAt:   time.Now().UTC(),
	})

	session.AddErrors([]aggregate.SessionError{
		{
			ID:         "err-1",
			SetNumber:  1,
			RepNumber:  1,
			ExerciseID: "ex-1",
			ErrorCode:  "ERR_1",
			Severity:   "HIGH",
			Timestamp:  time.Now().UTC(),
		},
	})

	// ToPersistence
	model := persistence.SessionToPersistence(session)
	if model.ID != "sess-1" || len(model.Sets) != 1 || len(model.Errors) != 1 {
		t.Fatalf("invalid persistence model conversion: %+v", model)
	}

	// ToDomain
	restoredSession := persistence.SessionToDomain(model)
	if restoredSession.ID() != "sess-1" {
		t.Errorf("got restored ID = %v, want sess-1", restoredSession.ID())
	}
	if len(restoredSession.Sets()) != 1 {
		t.Errorf("got sets count = %d, want 1", len(restoredSession.Sets()))
	}
	if len(restoredSession.Errors()) != 1 {
		t.Errorf("got errors count = %d, want 1", len(restoredSession.Errors()))
	}
}

func TestModels_PersonalRecordToPersistenceAndDomain(t *testing.T) {
	now := time.Now().UTC()
	pr := aggregate.ReconstitutePersonalRecord("pr-1", "user-1", "ex-1", 100.0, 100.0, 10, true, now, now, now)

	model := persistence.PersonalRecordToPersistence(pr)
	if model.ID != "pr-1" || model.UserID != "user-1" {
		t.Fatalf("invalid PR persistence model: %+v", model)
	}

	restoredPR := persistence.PersonalRecordToDomain(model)
	if restoredPR.ID() != "pr-1" || restoredPR.UserID() != "user-1" {
		t.Errorf("invalid PR domain model: %+v", restoredPR)
	}
}

func TestModels_OutboxToDomain(t *testing.T) {
	now := time.Now().UTC()
	model := &persistence.OutboxModel{
		ID:            "out-1",
		AggregateType: "WorkoutSession",
		AggregateID:   "sess-1",
		EventType:     "WorkoutSessionStarted",
		Payload:       []byte("{}"),
		Published:     true,
		CreatedAt:     now,
	}

	domainRecord := persistence.OutboxToDomain(model)
	if domainRecord.ID != "out-1" || !domainRecord.Published {
		t.Errorf("invalid domain outbox record: %+v", domainRecord)
	}
}
