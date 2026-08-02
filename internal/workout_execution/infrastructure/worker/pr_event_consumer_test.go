package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

type mockSessionRepo struct {
	session *aggregate.WorkoutSession
	findErr error
	saveErr error
}

var _ repository.WorkoutSessionRepository = (*mockSessionRepo)(nil)

func (m *mockSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	return m.saveErr
}

func (m *mockSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	return m.session, m.findErr
}

func (m *mockSessionRepo) FindTimedOutSessions(ctx context.Context, timeoutMinutes int) ([]*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindSessionsWithCriticalInactivity(ctx context.Context, threshold time.Duration) ([]*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

type mockPRRepo struct {
	err error
}

var _ repository.PersonalRecordRepository = (*mockPRRepo)(nil)

func (m *mockPRRepo) Save(ctx context.Context, pr *aggregate.PersonalRecord) error {
	return m.err
}

func (m *mockPRRepo) FindByUserIDAndExerciseID(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	return nil, m.err
}

func (m *mockPRRepo) FindByUserIDAndExerciseIDForUpdate(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	return nil, m.err
}

func (m *mockPRRepo) FindByUserIDAndExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {
	return nil, m.err
}

type mockTxManager struct {
	err error
}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

type mockOutboxWriter struct {
	err error
}

func (m *mockOutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	return m.err
}

func TestPREventConsumer(t *testing.T) {
	t.Run("NewPREventConsumer initialization", func(t *testing.T) {
		handler := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{}, &mockPRRepo{}, &mockOutboxWriter{}, &mockTxManager{})
		consumer := worker.NewPREventConsumer(handler)
		if consumer == nil {
			t.Fatal("got nil consumer, want non-nil")
		}
	})

	t.Run("OnWorkoutSessionCompleted session not found error", func(t *testing.T) {
		repoErr := errors.New("session db error")
		handler := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{findErr: repoErr},
			&mockPRRepo{},
			&mockOutboxWriter{},
			&mockTxManager{},
		)
		consumer := worker.NewPREventConsumer(handler)

		err := consumer.OnWorkoutSessionCompleted(context.Background(), "sess-1", "user-1")
		if err == nil {
			t.Fatal("got nil error, want error")
		}
	})

	t.Run("OnWorkoutSessionCompleted success PR process", func(t *testing.T) {
		session, err := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		formScore := float32(90.0)
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex-bench",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     80.0,
			FormScore:  &formScore,
			RPE:        8.0,
			CreatedAt:  time.Now().UTC(),
		})

		handler := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{},
			&mockOutboxWriter{},
			&mockTxManager{},
		)
		consumer := worker.NewPREventConsumer(handler)

		if err := consumer.OnWorkoutSessionCompleted(context.Background(), "sess-1", "user-1"); err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("HandleMessage valid CloudEvent", func(t *testing.T) {
		session, err := aggregate.NewWorkoutSession("sess-100", "user-100", "plan-1")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		handler := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{},
			&mockOutboxWriter{},
			&mockTxManager{},
		)
		consumer := worker.NewPREventConsumer(handler)

		rawJSON := []byte(`{
			"id": "evt-123",
			"type": "contracts.core.workout_execution.v1.workoutSessionCompleted",
			"data": {
				"sessionId": "sess-100",
				"userId": "user-100"
			}
		}`)

		if err := consumer.HandleMessage(context.Background(), rawJSON); err != nil {
			t.Fatalf("HandleMessage got err = %v, want nil", err)
		}
	})

	t.Run("HandleMessage non-matching event type ignored", func(t *testing.T) {
		handler := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{}, &mockPRRepo{}, &mockOutboxWriter{}, &mockTxManager{})
		consumer := worker.NewPREventConsumer(handler)

		rawJSON := []byte(`{
			"id": "evt-124",
			"type": "contracts.core.workout_execution.v1.workoutSessionStarted",
			"data": {"sessionId": "sess-100", "userId": "user-100"}
		}`)

		if err := consumer.HandleMessage(context.Background(), rawJSON); err != nil {
			t.Fatalf("HandleMessage non-matching type got err = %v, want nil", err)
		}
	})

	t.Run("HandleMessage invalid JSON payload", func(t *testing.T) {
		handler := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{}, &mockPRRepo{}, &mockOutboxWriter{}, &mockTxManager{})
		consumer := worker.NewPREventConsumer(handler)

		if err := consumer.HandleMessage(context.Background(), []byte(`invalid-json`)); err == nil {
			t.Fatal("HandleMessage invalid json got nil error, want error")
		}
	})

	t.Run("HandleMessage missing fields returns error", func(t *testing.T) {
		handler := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{}, &mockPRRepo{}, &mockOutboxWriter{}, &mockTxManager{})
		consumer := worker.NewPREventConsumer(handler)

		rawJSON := []byte(`{
			"id": "evt-125",
			"type": "contracts.core.workout_execution.v1.workoutSessionCompleted",
			"data": {"sessionId": "", "userId": ""}
		}`)

		if err := consumer.HandleMessage(context.Background(), rawJSON); err == nil {
			t.Fatal("HandleMessage missing fields got nil error, want error")
		}
	})
}
