//go:build unit

package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
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
	err      error
}

func (m *mockExerciseRepo) FindByID(ctx context.Context, id string) (*domain.Exercise, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.exercise == nil {
		return nil, domain.ErrExerciseNotFound
	}
	return m.exercise, nil
}

func (m *mockExerciseRepo) Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error {
	m.saved = exercise
	return nil
}

func TestSetAISupportedHandler_Success(t *testing.T) {
	now := time.Now()
	ex, err := domain.NewExercise(domain.Info{
		ID:             "ex-1",
		Name:           "Bench Press",
		BodyPartID:     "chest",
		EquipmentID:    "barbell",
		TargetMuscleID: "pectoralis",
	}, now)
	if err != nil {
		t.Fatalf("unexpected error creating exercise: %v", err)
	}

	repo := &mockExerciseRepo{exercise: ex}
	clock := mockClock{now: now}
	handler := command.NewSetAISupportedHandler(repo, clock)

	updated, err := handler.Handle(context.Background(), command.SetAISupportedCommand{
		ID:        "ex-1",
		Supported: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !updated.Info().HasAISupported {
		t.Errorf("expected HasAISupported to be true, got false")
	}
}
