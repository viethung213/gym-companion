package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
)

// Uses the shared mocks from the adapters package. Declaring separate ones here
// would validate exercise IDs against different data than cmd/api does.

// isLauncherInvocation reports whether args are meant for the ADK launcher.
//
// The launcher is addressed by subcommand ("console", "web", ...) and rejects
// any flag it does not define, so it cannot be handed this tool's own -user.
// Dispatching on "first argument is a subcommand, not a flag" keeps new
// launcher subcommands working without touching this list.
func isLauncherInvocation(args []string) bool {
	return len(args) > 0 && !strings.HasPrefix(args[0], "-")
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("test-coaching: %v", err)
	}
}

// run keeps the body out of main so every failure path returns an error and
// deferred cleanup still runs; log.Fatal mid-run would skip it.
func run() error {
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
		return fmt.Errorf("initialize agent: %w", err)
	}

	loader, err := adkagent.NewMultiLoader(
		coachAgent.Agent(),
		coachAgent.SuggestAdHocAgent(),
		coachAgent.RegeneratePendingAgent(),
		coachAgent.AdaptiveCycleAgent(),
	)
	if err != nil {
		return fmt.Errorf("create multi agent loader: %w", err)
	}

	cfg := launcher.Config{
		AgentLoader: loader,
	}

	args := os.Args[1:]

	if isLauncherInvocation(args) {
		l := full.NewLauncher()
		if execErr := l.Execute(ctx, &cfg, args); execErr != nil {
			return fmt.Errorf("run launcher: %w\n\n%s", execErr, l.CommandLineSyntax())
		}
		return nil
	}

	// A dedicated FlagSet, not the global one: the launcher parses os.Args with
	// its own flags on the other branch, and ContinueOnError keeps a bad flag
	// from calling os.Exit past the deferred span flush.
	fs := flag.NewFlagSet("test-coaching", flag.ContinueOnError)
	userID := fs.String("user", "test-user-123", "User ID to generate roadmap for")
	if parseErr := fs.Parse(args); parseErr != nil {
		return fmt.Errorf("parse flags: %w", parseErr)
	}

	log.Printf("Generating roadmap for user: %s\n", *userID)
	roadmap, err := coachAgent.GenerateRoadmap(ctx, *userID)
	if err != nil {
		// Deliberately not fatal: a failed generation still produced spans,
		// and those are often exactly what needs inspecting.
		log.Printf("Note: Execution error: %v\n", err)
		return nil
	}

	data, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal roadmap: %w", err)
	}
	fmt.Println("\n✅ Generated Roadmap:")
	fmt.Println(string(data))

	return nil
}
