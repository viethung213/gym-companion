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

// UserProfileClient provides integration with User Profile context (UC-03.4 A3).
type UserProfileClient interface {
	UpdateBodyWeight(ctx context.Context, userID string, weightKg float32) error
}
