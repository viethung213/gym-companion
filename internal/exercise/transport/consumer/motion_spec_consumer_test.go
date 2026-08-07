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
		name    string
		payload []byte
		wantErr bool
	}{
		{
			name:    "valid payload",
			payload: []byte(`{"exerciseId":"ex-100","isReady":true}`),
			wantErr: false,
		},
		{
			name:    "empty exerciseId",
			payload: []byte(`{"exerciseId":"","isReady":true}`),
			wantErr: true,
		},
		{
			name:    "invalid json",
			payload: []byte(`invalid json`),
			wantErr: true,
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

			if !tt.wantErr && !repo.saved.Info().HasAISupported {
				t.Errorf("expected HasAISupported to be true, got false")
			}
		})
	}
}
