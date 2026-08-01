package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
)

// Uses the shared mocks from the adapters package. Declaring separate ones here
// would validate exercise IDs against different data than cmd/api does.

func main() {
	ctx := context.Background()

	log.Println("Initializing Coaching Agent...")
	coachAgent, err := adk.NewCoachingContextAgent(
		ctx,
		&adapters.MockUserProfileReader{},
		&adapters.MockWorkoutSessionReader{},
		&adapters.MockExerciseCatalogReader{},
		nil, // no roadmap store: this tool only exercises fresh generation
		persistence.UUIDGenerator{},
	)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	loader, err := adkagent.NewMultiLoader(
		coachAgent.Agent(),
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
		if execErr := l.Execute(ctx, &cfg, os.Args[1:]); execErr != nil {
			log.Fatalf("Run failed: %v\n\n%s", execErr, l.CommandLineSyntax())
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

	data, _ := json.MarshalIndent(roadmap, "", "  ")
	fmt.Println("\n✅ Generated Roadmap:")
	fmt.Println(string(data))
}
