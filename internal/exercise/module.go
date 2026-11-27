package exercise

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/worker"
	exerciseConsumer "github.com/viethung213/gym-companion/internal/exercise/transport/consumer"
	exerciseGRPC "github.com/viethung213/gym-companion/internal/exercise/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*exerciseGRPC.ExerciseServer, func(), error) {

	// Initialize GORM DB wrapper over sql.DB with OpenTelemetry tracing
	gormDB, err := database.OpenGorm(deps.DB, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// Initialize Repositories
	repo := persistence.NewPostgresRepository(gormDB)
	outboxRepo := persistence.NewOutboxRepository(gormDB)

	// Initialize CQRS Command Handlers
	clock := persistence.SystemClock{}
	ids := persistence.RandomIDGenerator{}

	createHandler := command.NewCreateExerciseHandler(repo, clock, ids)
	updateHandler := command.NewUpdateExerciseHandler(repo, clock)
	approveHandler := command.NewApproveExerciseHandler(repo, clock, ids)
	archiveHandler := command.NewArchiveExerciseHandler(repo, clock, ids)
	restoreHandler := command.NewRestoreExerciseHandler(repo, clock, ids)

	// Initialize CQRS Query Handlers
	getHandler := query.NewGetExerciseHandler(repo)
	getRandomHandler := query.NewGetRandomExerciseHandler(repo)
	searchHandler := query.NewSearchExercisesHandler(repo)

	cfg := config.LoadConfig()

	// Initialize gRPC Handler and Register Service
	grpcHandler := exerciseGRPC.NewExerciseServer(
		createHandler,
		updateHandler,
		approveHandler,
		archiveHandler,
		restoreHandler,
		getHandler,
		getRandomHandler,
		searchHandler,
		cfg.MediaBaseURL,
	)

	// Start Background Worker for Outbox Pattern & Kafka
	writer, err := deps.KafkaRegistry.GetWriter("exercise.events")
	if err != nil {
		return nil, nil, fmt.Errorf("get exercise kafka writer: %w", err)
	}

	kafkaPub := kafka.NewPublisher(writer)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)

	// Initialize MotionSpec consumer for updating has_ai_supported
	setAISupportedHandler := command.NewSetAISupportedHandler(repo, clock)
	motionSpecConsumer := exerciseConsumer.NewMotionSpecConsumer(setAISupportedHandler)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in Exercise Outbox background worker", slog.Any("panic", r))
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	// Initialize and start Kafka Consumer Manager
	consumerMgr := exerciseConsumer.NewConsumerManager(deps.KafkaRegistry, motionSpecConsumer)
	if consumerMgr != nil {
		consumerMgr.Start(workerCtx)
	}

	// Shutdown callback function
	shutdown := func() {
		slog.Info("Shutting down Exercise Bounded Context background workers...")
		cancelWorkers()
		if consumerMgr != nil {
			consumerMgr.Wait()
		}
		wg.Wait()
		slog.Info("Exercise Bounded Context background workers stopped, closing Kafka publisher...")
		if err := kafkaPub.Close(); err != nil {
			slog.Warn("Failed to close exercise Kafka publisher", slog.String("error", err.Error()))
		}
		slog.Info("Exercise Bounded Context Kafka publisher closed successfully.")
	}

	slog.Info("Exercise Bounded Context initialized successfully.")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler registers the ConnectRPC handler for the Exercise module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	grpcServer *exerciseGRPC.ExerciseServer,
) string {
	connectHandler := exerciseGRPC.NewConnectExerciseHandler(grpcServer)
	exercisev1serviceconnect.RegisterExerciseServiceHandler(server, connectHandler)
	return "/" + exercisev1serviceconnect.ExerciseServiceName + "/"
}
