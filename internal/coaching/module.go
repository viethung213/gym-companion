package coaching

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	appEvent "github.com/viethung213/gym-companion/internal/coaching/application/event"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai"
	coachingKafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB              *sql.DB
	GRPCServer      *grpc.Server
	KafkaRegistry   *sharedKafka.Registry
	ExerciseService port.ExerciseQueryService
}

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	if isNilDependency(deps.ExerciseService) {
		return nil, errors.New("exercise query service is required")
	}
	if deps.DB == nil {
		return nil, errors.New("database is required")
	}
	if deps.GRPCServer == nil {
		return nil, errors.New("gRPC server is required")
	}
	if deps.KafkaRegistry == nil {
		return nil, errors.New("kafka registry is required")
	}

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
	unitOfWork := persistence.NewUnitOfWork(gormDB)
	clock := persistence.SystemClock{}
	ids := persistence.RandomIDGenerator{}

	planner := ai.NewMockPlanner()

	// 4. Command & Event Handlers
	initiateHandler := command.NewInitiateRoadmapHandler(
		roadmapRepo, scheduleRepo, planRepo,
		deps.ExerciseService, planner, clock, ids, unitOfWork,
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
	kafkaPub := coachingKafka.NewPublisher(writer, "coaching.events")
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)
	deadLetters := coachingKafka.NewDeadLetterPublisher(
		writer,
		"profile.events.dlq",
	)

	// Kafka Consumer for profile.events topic
	kafkaConsumer := coachingKafka.NewConsumer(
		kafkaBrokers,
		"profile.events",
		"coaching-group",
		inboxRepo,
		profileCompletedHandler,
		deadLetters,
	)

	// 7. Start Background Workers
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runManagedWorker(workerCtx, "Coaching Outbox", outboxWorker.Start)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runManagedWorker(workerCtx, "Coaching Kafka consumer", kafkaConsumer.Start)
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

func runManagedWorker(
	ctx context.Context,
	name string,
	run func(context.Context),
) {
	for ctx.Err() == nil {
		panicked := runWorkerOnce(ctx, name, run)
		if !panicked || !waitForWorkerRestart(ctx) {
			return
		}
	}
}

func runWorkerOnce(
	ctx context.Context,
	name string,
	run func(context.Context),
) (panicked bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = true
			log.Printf("PANIC RECOVERED in %s worker: %v", name, recovered)
		}
	}()
	run(ctx)

	return false
}

func waitForWorkerRestart(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
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
