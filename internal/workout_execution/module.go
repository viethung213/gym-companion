package workoutexecution

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
	workoutexecutionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	workoutEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ModuleDeps contains external dependencies required to boot the Workout Execution module.
type ModuleDeps struct {
	DB            *sql.DB
	GRPCServer    *grpc.Server
	KafkaRegistry *sharedKafka.Registry
}

// Initialize wires dependencies, registers gRPC services, and starts background workers.
func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// Initialize Repositories & Transaction Manager
	txManager := persistence.NewSQLTransactionManager(gormDB)
	sessionRepo := persistence.NewPostgresWorkoutSessionRepository(gormDB)
	prRepo := persistence.NewPostgresPersonalRecordRepository(gormDB)
	motionRepo := persistence.NewPostgresMotionSpecificationRepository(gormDB)
	outboxRepo := persistence.NewPostgresOutboxRepository(gormDB)

	outboxWriter := workoutEvent.NewOutboxWriter(outboxRepo)
	loadGuard := service.NewTrainingLoadGuard(sessionRepo)

	// Initialize Command Handlers
	startSessionHandler := command.NewStartWorkoutSessionHandler(sessionRepo, nil, outboxWriter, txManager)
	startScheduledSessionHandler := command.NewStartScheduledWorkoutSessionHandler(sessionRepo, outboxWriter, txManager)
	logSetHandler := command.NewLogWorkoutSetHandler(sessionRepo)
	completeSessionHandler := command.NewCompleteWorkoutSessionHandler(sessionRepo, loadGuard, nil, nil, outboxWriter, txManager)
	abortSessionHandler := command.NewAbortWorkoutSessionHandler(sessionRepo, outboxWriter, txManager)
	syncLogsHandler := command.NewSyncWorkoutLogsHandler(sessionRepo, outboxWriter, txManager)
	prProcessHandler := command.NewProcessCompletedSessionForPRHandler(sessionRepo, prRepo, outboxWriter, txManager)

	// Initialize Query Handlers
	getMotionSpecQuery := query.NewGetMotionSpecificationQueryHandler(motionRepo)
	getPRsQuery := query.NewGetPersonalRecordsQueryHandler(prRepo)
	getErrorsQuery := query.NewGetWorkoutSessionErrorsQueryHandler(sessionRepo)
	getHistoryQuery := query.NewGetWorkoutHistoryQueryHandler(sessionRepo)

	// Initialize gRPC Transport
	grpcHandler := transport.NewGRPCHandler(
		startSessionHandler,
		startScheduledSessionHandler,
		logSetHandler,
		completeSessionHandler,
		abortSessionHandler,
		syncLogsHandler,
		getMotionSpecQuery,
		getPRsQuery,
		getErrorsQuery,
		getHistoryQuery,
	)
	workoutexecutionv1service.RegisterWorkoutExecutionServiceServer(deps.GRPCServer, grpcHandler)
	workoutexecutionv1service.RegisterAdminWorkoutExecutionServiceServer(deps.GRPCServer, grpcHandler)

	// Kafka Setup
	kafkaBrokersStr := os.Getenv("WORKOUT_EXECUTION_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	_ = kafkaBrokers

	// Initialize Workers
	outboxWorker := worker.NewOutboxWorker(outboxRepo, nil, 2*time.Second)
	timeoutWorker := worker.NewSessionTimeoutWorker(sessionRepo, outboxWriter, txManager, 5*time.Minute)
	criticalWorker := worker.NewCriticalInactivityWorker(sessionRepo, outboxWriter, txManager, 1*time.Minute, 5*time.Minute)
	_ = worker.NewPREventConsumer(prProcessHandler)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in WorkoutExecution Outbox worker: %v", r)
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in WorkoutExecution Timeout worker: %v", r)
			}
		}()
		timeoutWorker.Start(workerCtx)
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in WorkoutExecution Critical Inactivity worker: %v", r)
			}
		}()
		criticalWorker.Start(workerCtx)
	}()

	shutdown := func() {
		log.Println("Shutting down Workout Execution Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		log.Println("Workout Execution Bounded Context background workers stopped successfully.")
	}

	log.Println("Workout Execution Bounded Context initialized successfully.")
	return shutdown, nil
}

// RegisterGateway registers gRPC-Gateway endpoints for REST proxy.
func RegisterGateway(
	ctx context.Context,
	mux *runtime.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	err := workoutexecutionv1service.RegisterWorkoutExecutionServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register workout execution service gateway handler: %w", err)
	}
	err = workoutexecutionv1service.RegisterAdminWorkoutExecutionServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register admin workout execution service gateway handler: %w", err)
	}
	return nil
}
