package coaching

import (
	"context"
	"fmt"
	"log"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	"github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
)

// ModuleDeps contains dependencies for coaching module initialization.
type ModuleDeps struct {
	GRPCServer    *grpc.Server
	KafkaRegistry *kafka.Registry
	ProfileReader port.UserProfileReader
	SessionReader port.WorkoutSessionReader
	CatalogReader port.ExerciseCatalogReader
}

// Initialize sets up the coaching module (coaching context agent, gRPC services, etc).
func Initialize(ctx context.Context, deps ModuleDeps) (port.CoachAgent, func(), error) {
	log.Println("Initializing Coaching Module...")

	// Initialize Coaching Context Agent (ADK)
	coachAgent, err := adk.NewCoachingContextAgent(
		ctx,
		deps.ProfileReader,
		deps.SessionReader,
		deps.CatalogReader,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize coaching context agent: %w", err)
	}

	log.Println("✅ Coaching Module initialized successfully")

	shutdown := func() {
		log.Println("Shutting down Coaching Module...")
	}

	return coachAgent, shutdown, nil
}
