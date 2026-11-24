package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type mockRandomRepo struct {
	exercise   *domain.Exercise
	err        error
	calledWith string
}

func (m *mockRandomRepo) Save(_ context.Context, _ *domain.Exercise, _ *domain.Event) error {
	return nil
}

func (m *mockRandomRepo) FindByID(_ context.Context, _ string) (*domain.Exercise, error) {
	return nil, nil
}

func (m *mockRandomRepo) SearchActive(_ context.Context, _ *port.SearchFilters) ([]*domain.Exercise, int32, error) {
	return nil, 0, nil
}

func (m *mockRandomRepo) SetAISupportedWithOutboxLog(_ context.Context, _ string, _ bool, _ *port.OutboxLogRecord) error {
	return nil
}

func (m *mockRandomRepo) GetRandomAISupported(_ context.Context) (*domain.Exercise, error) {
	return m.exercise, m.err
}

func TestGetRandomExerciseHandler(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	ex, err := domain.NewExercise(domain.Info{
		ID:              "ex-pushup",
		Slug:            "Pushups",
		Name:            "Push-Up",
		PrimaryMuscle:   domain.MuscleGroupChest,
		Equipment:       domain.EquipmentBodyOnly,
		Category:        domain.CategoryStrength,
		Level:           domain.LevelBeginner,
		Source:          domain.SourceFreeExerciseDB,
		Instructions:    []string{"Keep body straight and push"},
		MotionSlug:      "push_up",
		HasAISupported:  true,
		MeasurementType: domain.MeasurementTypeReps,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error creating exercise: %v", err)
	}

	t.Run("success returning random exercise", func(t *testing.T) {
		t.Parallel()
		repo := &mockRandomRepo{exercise: ex}
		handler := query.NewGetRandomExerciseHandler(repo)

		got, err := handler.Handle(context.Background(), query.GetRandomExerciseQuery{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Info().ID != "ex-pushup" {
			t.Errorf("got ID %q, want %q", got.Info().ID, "ex-pushup")
		}
	})

	t.Run("not found error", func(t *testing.T) {
		t.Parallel()
		repo := &mockRandomRepo{err: domain.ErrExerciseNotFound}
		handler := query.NewGetRandomExerciseHandler(repo)

		_, err := handler.Handle(context.Background(), query.GetRandomExerciseQuery{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
