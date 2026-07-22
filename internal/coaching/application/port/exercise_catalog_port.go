package port

import "context"

type ExerciseMetadata struct {
	ID                 string
	Name               string
	Category           string
	PrimaryMuscleGroup string
	EquipmentID        string
	IsCompound         bool
}

type ExerciseCatalogPort interface {
	GetExerciseByID(ctx context.Context, exerciseID string) (*ExerciseMetadata, error)
	FilterExercisesByEquipment(ctx context.Context, equipmentIDs []string) ([]ExerciseMetadata, error)
}
