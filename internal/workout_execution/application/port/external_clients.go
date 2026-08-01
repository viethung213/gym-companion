package port

import "context"

// DailyWorkoutPlanClient provides integration with the Workout Plan context (UC-03.1).
type DailyWorkoutPlanClient interface {
	ValidatePlanExists(ctx context.Context, userID, planID string) (bool, error)
}

// ExerciseCatalogClient provides integration with Exercise Catalog context.
type ExerciseCatalogClient interface {
	GetExerciseMuscleGroup(ctx context.Context, exerciseID string) (string, error)
}
