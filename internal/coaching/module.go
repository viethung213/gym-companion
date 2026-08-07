package coaching

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	coachingEvent "github.com/viethung213/gym-companion/internal/coaching/infrastructure/event"
	coachingKafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	coachingGrpc "github.com/viethung213/gym-companion/internal/coaching/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service/coachingv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/kafka"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ModuleDeps contains dependencies for coaching module initialization.
type ModuleDeps struct {
	DB            *sql.DB
	GormDB        *gorm.DB
	KafkaRegistry *kafka.Registry
	ProfileReader port.UserProfileReader
	SessionReader port.WorkoutSessionReader
	CatalogReader port.ExerciseCatalogReader
	RoadmapRepo   port.RoadmapRepository
	OutboxWriter  port.OutboxWriter
	IDGenerator   port.IDGenerator
}

// Initialize sets up the coaching module (coaching context agent, gRPC services, etc).
func Initialize(ctx context.Context, deps *ModuleDeps) (port.CoachAgent, func(), error) {
	log.Println("Initializing Coaching Module...")

	if deps.IDGenerator == nil {
		deps.IDGenerator = persistence.UUIDGenerator{}
	}

	var gormDB *gorm.DB = deps.GormDB
	if gormDB == nil && deps.DB != nil {
		var err error
		gormDB, err = gorm.Open(gormPostgres.New(gormPostgres.Config{Conn: deps.DB}), &gorm.Config{})
		if err != nil {
			log.Printf("[Coaching Module] Warning: Failed to wrap *sql.DB into GORM: %v", err)
		}
	}

	if deps.RoadmapRepo == nil && gormDB != nil {
		deps.RoadmapRepo = persistence.NewRoadmapRepository(gormDB)
	}

	var outboxRepo port.OutboxRepository
	if gormDB != nil {
		outboxRepo = persistence.NewOutboxRepository(gormDB)
	}

	if deps.OutboxWriter == nil && outboxRepo != nil {
		deps.OutboxWriter = coachingEvent.NewOutboxWriter(outboxRepo)
	}

	// Initialize Coaching Context Agent (ADK)
	coachAgent, err := adk.NewCoachingContextAgent(
		ctx,
		deps.ProfileReader,
		deps.SessionReader,
		deps.CatalogReader,
		deps.RoadmapRepo,
		deps.IDGenerator,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize coaching context agent: %w", err)
	}

	var reminderWorker *worker.UpcomingWorkoutReminderWorker
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil {
		reminderWorker = worker.NewUpcomingWorkoutReminderWorker(deps.RoadmapRepo, deps.OutboxWriter, 5*time.Minute)
		go func() {
			_ = reminderWorker.Start(ctx)
		}()
		log.Println("✅ Coaching UpcomingWorkoutReminderWorker started")
	} else {
		log.Println("⚠️ Coaching UpcomingWorkoutReminderWorker not started (missing RoadmapRepo or OutboxWriter)")
	}

	var outboxWorker *worker.OutboxWorker
	if outboxRepo != nil && deps.KafkaRegistry != nil {
		brokersStr := os.Getenv("KAFKA_BROKERS")
		if brokersStr == "" {
			brokersStr = "localhost:9092"
		}
		brokers := strings.Split(brokersStr, ",")

		writer, wErr := deps.KafkaRegistry.GetWriter("events.v1", brokers)
		if wErr == nil && writer != nil {
			kafkaPub := coachingKafka.NewPublisher(writer)
			outboxWorker = worker.NewOutboxWorker(outboxRepo, kafkaPub, 2*time.Second)
			go outboxWorker.Start(ctx)
			log.Println("✅ Coaching OutboxWorker started")
		}
	}

	log.Println("✅ Coaching Module initialized successfully")

	shutdown := func() {
		log.Println("Shutting down Coaching Module...")
		if reminderWorker != nil {
			reminderWorker.Stop()
		}
	}

	return coachAgent, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for Coaching module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	server *coachingGrpc.Server,
	opts ...connect.HandlerOption,
) {
	if server == nil {
		return
	}
	connectHandler := coachingGrpc.NewConnectCoachingHandler(server)
	path, handler := coachingv1serviceconnect.NewCoachingServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
