package adapters

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// MockUserProfileReader provides a mock implementation for testing.
type MockUserProfileReader struct{}

func (m *MockUserProfileReader) GetProfile(_ context.Context, userID string) (port.Profile, error) {
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

func (m *MockWorkoutSessionReader) GetRecentSessions(_ context.Context, _ string, _ time.Time) ([]port.WorkoutSession, error) {
	return []port.WorkoutSession{}, nil
}

func (m *MockWorkoutSessionReader) GetSetLogs(_ context.Context, _, _ string, _ int) ([]port.SetLog, error) {
	// Return mock PR data
	return []port.SetLog{
		{ExerciseID: "bench-press", Weight: 80.0, Reps: 8},
		{ExerciseID: "bench-press", Weight: 85.0, Reps: 5},
		{ExerciseID: "squat", Weight: 110.0, Reps: 5},
		{ExerciseID: "barbell-row", Weight: 72.0, Reps: 8},
	}, nil
}

func getMockExerciseCatalog() map[string][]port.Exercise {
	return map[string][]port.Exercise{
		"chest": {
			{ExerciseID: "bench-press", Name: "Bench Press", MuscleGroup: "chest", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "push-up", Name: "Push Up", MuscleGroup: "chest", Equipment: "", IsMachineOrCable: false},
			{ExerciseID: "db-press", Name: "Dumbbell Press", MuscleGroup: "chest", Equipment: "dumbbell", IsMachineOrCable: false},
			{ExerciseID: "cable-fly", Name: "Cable Fly", MuscleGroup: "chest", Equipment: "cable", IsMachineOrCable: true},
			{ExerciseID: "machine-chest-fly", Name: "Machine Chest Fly", MuscleGroup: "chest", Equipment: "machine", IsMachineOrCable: true},
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
			{ExerciseID: "seated-leg-curl", Name: "Seated Leg Curl", MuscleGroup: "legs", Equipment: "machine", IsMachineOrCable: true},
		},
		"shoulders": {
			{ExerciseID: "overhead-press", Name: "Overhead Press", MuscleGroup: "shoulders", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "lateral-raise", Name: "Lateral Raise", MuscleGroup: "shoulders", Equipment: "dumbbell", IsMachineOrCable: false},
		},
	}
}

// MockExerciseCatalogReader provides a mock implementation for testing.
type MockExerciseCatalogReader struct{}

func (m *MockExerciseCatalogReader) SearchByFilter(_ context.Context, filter *port.ExerciseFilter) ([]port.Exercise, error) {
	mg := filter.TargetMuscleID
	if mg == "" {
		mg = filter.BodyPartID
	}

	catalog := getMockExerciseCatalog()
	result, ok := catalog[mg]
	if !ok {
		// Unknown group: return everything, sorted so results stay deterministic.
		groups := make([]string, 0, len(catalog))
		for g := range catalog {
			groups = append(groups, g)
		}
		sort.Strings(groups)

		var all []port.Exercise
		for _, g := range groups {
			all = append(all, catalog[g]...)
		}
		result = all
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

// GetByID never echoes unknown IDs back; that would make ID validation vacuous.
func (m *MockExerciseCatalogReader) GetByID(_ context.Context, exerciseID string) (port.Exercise, error) {
	for _, list := range getMockExerciseCatalog() {
		for _, ex := range list {
			if ex.ExerciseID == exerciseID {
				return ex, nil
			}
		}
	}
	return port.Exercise{}, fmt.Errorf("%w: %s", port.ErrExerciseNotFound, exerciseID)
}

// Compile-time interface checks
var (
	_ port.UserProfileReader     = (*MockUserProfileReader)(nil)
	_ port.WorkoutSessionReader  = (*MockWorkoutSessionReader)(nil)
	_ port.ExerciseCatalogReader = (*MockExerciseCatalogReader)(nil)
)
