package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
)

// mockProfileReader implements port.UserProfileReader
type mockProfileReader struct{}

func (m *mockProfileReader) GetProfile(ctx context.Context, userID string) (port.Profile, error) {
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

// mockSessionReader implements port.WorkoutSessionReader
type mockSessionReader struct{}

func (m *mockSessionReader) GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]port.WorkoutSession, error) {
	return []port.WorkoutSession{}, nil
}

func (m *mockSessionReader) GetSetLogs(ctx context.Context, userID, exerciseID string, limit int) ([]port.SetLog, error) {
	return []port.SetLog{
		{ExerciseID: "bench-press", Weight: 80.0, Reps: 8},
		{ExerciseID: "bench-press", Weight: 85.0, Reps: 5},
	}, nil
}

// mockCatalogReader implements port.ExerciseCatalogReader
type mockCatalogReader struct{}

func (m *mockCatalogReader) SearchByFilter(ctx context.Context, filter port.ExerciseFilter) ([]port.Exercise, error) {
	exercises := map[string][]port.Exercise{
		"chest": {
			{ExerciseID: "bench-press", Name: "Bench Press", MuscleGroup: "chest", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "push-up", Name: "Push Up", MuscleGroup: "chest", Equipment: "", IsMachineOrCable: false},
			{ExerciseID: "db-press", Name: "Dumbbell Press", MuscleGroup: "chest", Equipment: "dumbbell", IsMachineOrCable: false},
		},
		"back": {
			{ExerciseID: "barbell-row", Name: "Barbell Row", MuscleGroup: "back", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "pull-up", Name: "Pull Up", MuscleGroup: "back", Equipment: "pull-up-bar", IsMachineOrCable: false},
		},
		"legs": {
			{ExerciseID: "squat", Name: "Back Squat", MuscleGroup: "legs", Equipment: "barbell", IsMachineOrCable: false},
			{ExerciseID: "rdl", Name: "RDL", MuscleGroup: "legs", Equipment: "barbell", IsMachineOrCable: false},
		},
	}

	mg := filter.TargetMuscleID
	if mg == "" {
		mg = filter.BodyPartID
	}

	result := exercises[mg]
	if len(result) == 0 {
		for _, list := range exercises {
			result = append(result, list...)
		}
	}
	if len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (m *mockCatalogReader) GetByID(ctx context.Context, exerciseID string) (port.Exercise, error) {
	return port.Exercise{
		ExerciseID:       exerciseID,
		Name:             exerciseID,
		MuscleGroup:      "chest",
		Equipment:        "barbell",
		IsMachineOrCable: false,
	}, nil
}

// Compile-time interface assertions
var (
	_ port.UserProfileReader     = (*mockProfileReader)(nil)
	_ port.WorkoutSessionReader  = (*mockSessionReader)(nil)
	_ port.ExerciseCatalogReader = (*mockCatalogReader)(nil)
)

func main() {
	ctx := context.Background()

	log.Println("Initializing Coaching Agent...")
	coachAgent, err := adk.NewCoachingContextAgent(
		ctx,
		&mockProfileReader{},
		&mockSessionReader{},
		&mockCatalogReader{},
	)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	loader, err := adkagent.NewMultiLoader(
		coachAgent.Agent(),
		coachAgent.DefaultAgent(),
		coachAgent.SuggestAdHocAgent(),
		coachAgent.RegeneratePendingAgent(),
		coachAgent.AdaptiveCycleAgent(),
	)
	if err != nil {
		log.Fatalf("Failed to create multi agent loader: %v", err)
	}

	cfg := launcher.Config{
		AgentLoader: loader,
	}

	l := full.NewLauncher()
	if len(os.Args) > 1 {
		if err := l.Execute(ctx, &cfg, os.Args[1:]); err != nil {
			log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
		}
		return
	}

	userID := flag.String("user", "test-user-123", "User ID to generate roadmap for")
	flag.Parse()

	log.Printf("Generating roadmap for user: %s\n", *userID)
	roadmap, err := coachAgent.GenerateRoadmap(ctx, *userID)
	if err != nil {
		log.Printf("Note: Execution error: %v\n", err)
		return
	}

	// Pretty print result
	data, _ := json.MarshalIndent(roadmap, "", "  ")
	fmt.Println("\n✅ Generated Roadmap:")
	fmt.Println(string(data))
}
