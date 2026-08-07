package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
)

// Uses the shared mocks from the adapters package. Declaring separate ones here
// would validate exercise IDs against different data than cmd/api does.

// telemetryFlushTimeout bounds the final span export on the way out.
const telemetryFlushTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("test-coaching: %v", err)
	}
}

// run owns the whole lifecycle so the telemetry shutdown actually runs; a
// log.Fatal anywhere below would skip the deferred flush and drop the spans
// of the very invocation being measured.
func run() error {
	ctx := context.Background()

	shutdownTelemetry, traced, err := telemetry.Setup(ctx, "gym-companion-coaching")
	if err != nil {
		return fmt.Errorf("setup telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), telemetryFlushTimeout)
		defer cancel()
		if shutdownErr := shutdownTelemetry(shutdownCtx); shutdownErr != nil {
			log.Printf("telemetry shutdown: %v", shutdownErr)
		}
	}()
	if traced {
		log.Printf("Tracing enabled → %s", os.Getenv(telemetry.TracesEndpointEnv))
	}

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

	l := full.NewLauncher()
	if len(os.Args) > 1 {
		if execErr := l.Execute(ctx, &cfg, os.Args[1:]); execErr != nil {
			return fmt.Errorf("run launcher: %w\n\n%s", execErr, l.CommandLineSyntax())
		}
		return nil
	}

	userID := flag.String("user", "test-user-123", "User ID to generate roadmap for")
	flag.Parse()

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
