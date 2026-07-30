package adapters

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// MockUserProfileReader provides a mock implementation for testing.
type MockUserProfileReader struct{}

func (m *MockUserProfileReader) GetProfile(ctx context.Context, userID string) (port.Profile, error) {
	return port.Profile{
		UserID:      userID,
		WeightKg:    75.0,
		PrimaryGoal: "hypertrophy",
		AvailableEquipment: []string{
			"barbell", "bench", "cable", "dumbbell", "pull-up-bar", "rack",
		},
		PreferredMuscleGroups: []string{"chest", "back", "legs", "shoulders"},
		AvailableSlots: []port.Slot{
			{DayOfWeek: 0, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: 2, StartTime: "06:00", EndTime: "07:30"},
			{DayOfWeek: 4, StartTime: "06:00", EndTime: "07:30"},
		},
		ActiveInjuries: []port.InjuryStatus{},
	}, nil
}

// MockWorkoutSessionReader provides a mock implementation for testing.
type MockWorkoutSessionReader struct{}

func (m *MockWorkoutSessionReader) GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]port.WorkoutSession, error) {
	return []port.WorkoutSession{}, nil
}

func (m *MockWorkoutSessionReader) GetSetLogs(ctx context.Context, userID, exerciseID string, limit int) ([]port.SetLog, error) {
	// Return mock PR data
	return []port.SetLog{
		{ExerciseID: "bench-press", Weight: 80.0, Reps: 8},
		{ExerciseID: "bench-press", Weight: 85.0, Reps: 5},
		{ExerciseID: "squat", Weight: 110.0, Reps: 5},
		{ExerciseID: "barbell-row", Weight: 72.0, Reps: 8},
	}, nil
}

// MockExerciseCatalogReader provides a mock implementation for testing.
type MockExerciseCatalogReader struct{}

func (m *MockExerciseCatalogReader) SearchByFilter(ctx context.Context, filter port.ExerciseFilter) ([]port.Exercise, error) {
	exercises := map[string][]port.Exercise{
		"chest": {
			{ExerciseID: "bench-press", Name: "Bench Press", MuscleGroup: "chest", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "push-up", Name: "Push Up", MuscleGroup: "chest", Equipment: "", IsMachineOrCable: false},
			{ExerciseID: "db-press", Name: "Dumbbell Press", MuscleGroup: "chest", Equipment: "dumbbell", IsMachineOrCable: false},
			{ExerciseID: "cable-fly", Name: "Cable Fly", MuscleGroup: "chest", Equipment: "cable", IsMachineOrCable: true},
		},
		"back": {
			{ExerciseID: "barbell-row", Name: "Barbell Row", MuscleGroup: "back", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "pull-up", Name: "Pull Up", MuscleGroup: "back", Equipment: "pull-up-bar", IsMachineOrCable: false},
			{ExerciseID: "lat-pulldown", Name: "Lat Pulldown", MuscleGroup: "back", Equipment: "cable", IsMachineOrCable: true},
		},
		"legs": {
			{ExerciseID: "squat", Name: "Back Squat", MuscleGroup: "legs", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "rdl", Name: "RDL", MuscleGroup: "legs", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "lunge", Name: "Lunge", MuscleGroup: "legs", Equipment: "", IsMachineOrCable: false},
		},
		"shoulders": {
			{ExerciseID: "overhead-press", Name: "Overhead Press", MuscleGroup: "shoulders", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "lateral-raise", Name: "Lateral Raise", MuscleGroup: "shoulders", Equipment: "dumbbell", IsMachineOrCable: false},
		},
	}

	mg := filter.TargetMuscleID
	if mg == "" {
		mg = filter.BodyPartID
	}

	result, ok := exercises[mg]
	if !ok {
		// Return all exercises if specific muscle group not found
		var all []port.Exercise
		for _, list := range exercises {
			all = append(all, list...)
		}
		result = all
	}

	if len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (m *MockExerciseCatalogReader) GetByID(ctx context.Context, exerciseID string) (port.Exercise, error) {
	return port.Exercise{
		ExerciseID:       exerciseID,
		Name:             exerciseID,
		MuscleGroup:      "chest",
		Equipment:        "barbell",
		IsMachineOrCable: false,
	}, nil
}

// Compile-time interface checks
var (
	_ port.UserProfileReader     = (*MockUserProfileReader)(nil)
	_ port.WorkoutSessionReader  = (*MockWorkoutSessionReader)(nil)
	_ port.ExerciseCatalogReader = (*MockExerciseCatalogReader)(nil)
)
