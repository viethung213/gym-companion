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

func (m *mockRandomRepo) GetMetadata(_ context.Context) (port.Metadata, error) {
	return port.Metadata{}, nil
}

func (m *mockRandomRepo) CreateBodyPart(_ context.Context, _ *port.BodyPart) error {
	return nil
}

func (m *mockRandomRepo) GetBodyPart(_ context.Context, _ string) (*port.BodyPart, error) {
	return nil, nil
}

func (m *mockRandomRepo) ListBodyParts(_ context.Context, _, _ int) ([]port.BodyPart, int, error) {
	return nil, 0, nil
}

func (m *mockRandomRepo) UpdateBodyPart(_ context.Context, _ *port.BodyPart) error {
	return nil
}

func (m *mockRandomRepo) DeleteBodyPart(_ context.Context, _ string) error {
	return nil
}

func (m *mockRandomRepo) CreateEquipment(_ context.Context, _ *port.Equipment) error {
	return nil
}

func (m *mockRandomRepo) GetEquipment(_ context.Context, _ string) (*port.Equipment, error) {
	return nil, nil
}

func (m *mockRandomRepo) ListEquipments(_ context.Context, _, _ int) ([]port.Equipment, int, error) {
	return nil, 0, nil
}

func (m *mockRandomRepo) UpdateEquipment(_ context.Context, _ *port.Equipment) error {
	return nil
}

func (m *mockRandomRepo) DeleteEquipment(_ context.Context, _ string) error {
	return nil
}

func (m *mockRandomRepo) CreateMuscle(_ context.Context, _ *port.Muscle) error {
	return nil
}

func (m *mockRandomRepo) GetMuscle(_ context.Context, _ string) (*port.Muscle, error) {
	return nil, nil
}

func (m *mockRandomRepo) ListMuscles(_ context.Context, _ string, _, _ int) ([]port.Muscle, int, error) {
	return nil, 0, nil
}

func (m *mockRandomRepo) UpdateMuscle(_ context.Context, _ *port.Muscle) error {
	return nil
}

func (m *mockRandomRepo) DeleteMuscle(_ context.Context, _ string) error {
	return nil
}

func (m *mockRandomRepo) CreateTag(_ context.Context, _ *port.Tag) error {
	return nil
}

func (m *mockRandomRepo) GetTag(_ context.Context, _ string) (*port.Tag, error) {
	return nil, nil
}

func (m *mockRandomRepo) ListTags(_ context.Context, _, _ int) ([]port.Tag, int, error) {
	return nil, 0, nil
}

func (m *mockRandomRepo) UpdateTag(_ context.Context, _ *port.Tag) error {
	return nil
}

func (m *mockRandomRepo) DeleteTag(_ context.Context, _ string) error {
	return nil
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
		Name:            "Push-Up",
		BodyPartID:      "bp-chest",
		EquipmentID:     "eq-bodyweight",
		TargetMuscleID:  "m-chest",
		Instructions:    "Keep body straight and push",
		Difficulty:      "BEGINNER",
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
