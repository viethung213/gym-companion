package coaching

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	appEvent "github.com/viethung213/gym-companion/internal/coaching/application/event"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai"
	coachingGrpc "github.com/viethung213/gym-companion/internal/coaching/infrastructure/grpc"
	coachingKafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB            *sql.DB
	GRPCServer    *grpc.Server
	KafkaRegistry *sharedKafka.Registry
	ExerciseClient exercisesvc.ExerciseServiceClient // Optional gRPC client
}

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	// 1. Wrap sql.DB with GORM
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap db in gorm: %w", err)
	}

	// 2. Repositories
	roadmapRepo := persistence.NewPostgresRoadmapRepository(gormDB)
	scheduleRepo := persistence.NewPostgresWeeklyScheduleRepository(gormDB)
	planRepo := persistence.NewPostgresDailyWorkoutPlanRepository(gormDB)
	inboxRepo := persistence.NewPostgresInboxRepository(gormDB)
	outboxRepo := persistence.NewOutboxRepository(gormDB)
	clock := persistence.SystemClock{}
	ids := persistence.RandomIDGenerator{}

	// 3. Infrastructure Adapters
	var exerciseAdapter *coachingGrpc.ExerciseAdapter
	if deps.ExerciseClient != nil {
		exerciseAdapter = coachingGrpc.NewExerciseAdapter(deps.ExerciseClient)
	}

	planner := ai.NewMockPlanner()

	// 4. Command & Event Handlers
	initiateHandler := command.NewInitiateRoadmapHandler(
		roadmapRepo, scheduleRepo, planRepo,
		exerciseAdapter, planner, clock, ids,
	)
	profileCompletedHandler := appEvent.NewProfileCompletedHandler(initiateHandler)

	// 5. Register gRPC Server
	grpcHandler := transport.NewCoachingServer(initiateHandler)
	coachingsvc.RegisterCoachingServiceServer(deps.GRPCServer, grpcHandler)

	// 6. Kafka Setup
	kafkaBrokersStr := os.Getenv("COACHING_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	// Kafka Outbox Publisher
	writer, err := deps.KafkaRegistry.GetWriter("coaching", kafkaBrokers)
	if err != nil {
		return nil, fmt.Errorf("get coaching kafka writer: %w", err)
	}
	kafkaPub := coachingKafka.NewPublisher(writer)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)

	// Kafka Consumer for profile.events topic
	kafkaConsumer := coachingKafka.NewConsumer(
		kafkaBrokers,
		"profile.events",
		"coaching-group",
		inboxRepo,
		profileCompletedHandler,
	)

	// 7. Start Background Workers
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in Coaching Outbox background worker: %v", r)
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in Coaching Kafka consumer: %v", r)
			}
		}()
		kafkaConsumer.Start(workerCtx)
	}()

	// 8. Shutdown Callback
	shutdown := func() {
		log.Println("Shutting down Coaching Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		_ = kafkaConsumer.Close()
		_ = kafkaPub.Close()
		log.Println("Coaching Bounded Context background workers stopped successfully.")
	}

	log.Println("Coaching Bounded Context initialized successfully.")
	return shutdown, nil
}

func RegisterGateway(
	ctx context.Context,
	mux *runtime.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	err := coachingsvc.RegisterCoachingServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register coaching service gateway handler: %w", err)
	}
	return nil
}
