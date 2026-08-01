//go:build unit

package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/worker"
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

	repo := &mockExerciseRepo{exercise: ex}
	handler := command.NewSetAISupportedHandler(repo, mockClock{now: now})
	consumer := worker.NewMotionSpecConsumer(handler)

	payload := []byte(`{"exerciseId":"ex-100","isReady":true}`)
	if err := consumer.ConsumeMotionSpecReady(context.Background(), payload); err != nil {
		t.Fatalf("expected no error consuming event, got %v", err)
	}

	if !repo.saved.Info().HasAISupported {
		t.Errorf("expected HasAISupported to be true, got false")
	}
}
