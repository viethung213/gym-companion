package adapter

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

var _ port.ExerciseCatalogPort = (*MockExerciseCatalogAdapter)(nil)

// MockExerciseCatalogAdapter is a placeholder adapter for Exercise Catalog Port.
type MockExerciseCatalogAdapter struct{}

func NewMockExerciseCatalogAdapter() *MockExerciseCatalogAdapter {
	return &MockExerciseCatalogAdapter{}
}

func (m *MockExerciseCatalogAdapter) GetExerciseByID(ctx context.Context, exerciseID string) (*port.ExerciseMetadata, error) {
	return &port.ExerciseMetadata{
		ID:                 exerciseID,
		Name:               "Mock Exercise",
		Category:           "Compound",
		PrimaryMuscleGroup: "Chest",
		EquipmentID:        "eq-dumbbell",
		IsCompound:         true,
	}, nil
}

func (m *MockExerciseCatalogAdapter) FilterExercisesByEquipment(ctx context.Context, equipmentIDs []string) ([]port.ExerciseMetadata, error) {
	return []port.ExerciseMetadata{
		{
			ID:                 "ex-123",
			Name:               "Mock Exercise 1",
			Category:           "Compound",
			PrimaryMuscleGroup: "Chest",
			EquipmentID:        "eq-dumbbell",
			IsCompound:         true,
		},
	}, nil
}

