//go:build unit

package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/exercise/transport/consumer"
)

type mockClock struct {
	now time.Time
}

func (m mockClock) Now() time.Time {
	return m.now
}

type mockExerciseRepo struct {
	port.Repository
	exercise *domain.Exercise
	saved    *domain.Exercise
}

func (m *mockExerciseRepo) FindByID(ctx context.Context, id string) (*domain.Exercise, error) {
	if m.exercise == nil {
		return nil, domain.ErrExerciseNotFound
	}
	return m.exercise, nil
}

func (m *mockExerciseRepo) Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error {
	m.saved = exercise
	return nil
}

func (m *mockExerciseRepo) SetAISupportedWithOutboxLog(ctx context.Context, exerciseID string, supported bool, logRecord *port.OutboxLogRecord) error {
	if m.exercise != nil {
		_ = m.exercise.SetAISupported(supported, time.Now())
		m.saved = m.exercise
	}
	return nil
}

func TestMotionSpecConsumer_ConsumeMotionSpecReady(t *testing.T) {
	now := time.Now()
	ex, err := domain.NewExercise(domain.Info{
		ID:             "ex-100",
		Name:           "Squat",
		BodyPartID:     "legs",
		EquipmentID:    "barbell",
		TargetMuscleID: "quadriceps",
	}, now)
	if err != nil {
		t.Fatalf("unexpected error creating exercise: %v", err)
	}

	tests := []struct {
		name        string
		payload     []byte
		wantErr     bool
		wantUpdated bool
	}{
		{
			name:        "valid payload",
			payload:     []byte(`{"ExerciseID":"ex-100","IsReady":true}`),
			wantErr:     false,
			wantUpdated: true,
		},
		{
			name:        "valid cloudevent payload with PascalCase keys",
			payload:     []byte(`{"specversion":"1.0","type":"contracts.core.workout_execution.v1.motionSpecificationUpdated","data":{"ExerciseID":"ex-100","IsReady":true}}`),
			wantErr:     false,
			wantUpdated: true,
		},
		{
			name:        "valid cloudevent payload with camelCase keys",
			payload:     []byte(`{"specversion":"1.0","type":"contracts.core.workout_execution.v1.motionSpecificationUpdated","data":{"exerciseId":"ex-100","isReady":true}}`),
			wantErr:     false,
			wantUpdated: true,
		},
		{
			name:        "ignore unrelated event on shared topic",
			payload:     []byte(`{"specversion":"1.0","type":"contracts.core.workout_execution.v1.workoutSessionStarted","data":{"sessionId":"s-1","userId":"u-1"}}`),
			wantErr:     false,
			wantUpdated: false,
		},
		{
			name:        "empty exerciseId",
			payload:     []byte(`{"exerciseId":"","isReady":true}`),
			wantErr:     true,
			wantUpdated: false,
		},
		{
			name:        "invalid json",
			payload:     []byte(`invalid json`),
			wantErr:     true,
			wantUpdated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockExerciseRepo{exercise: ex}
			handler := command.NewSetAISupportedHandler(repo, mockClock{now: now})
			consumer := consumer.NewMotionSpecConsumer(handler)

			err := consumer.ConsumeMotionSpecReady(context.Background(), tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConsumeMotionSpecReady() got error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantUpdated {
				if repo.saved == nil || !repo.saved.Info().HasAISupported {
					t.Errorf("expected HasAISupported to be true, got repo.saved=%v", repo.saved)
				}
			} else if repo.saved != nil {
				t.Errorf("expected repo.saved to be nil for ignored or errored event, got %v", repo.saved)
			}
		})
	}
}
