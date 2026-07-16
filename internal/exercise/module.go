package exercise

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
	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/worker"
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
}

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	// Initialize GORM DB wrapper over sql.DB
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// Initialize Repositories
	repo := persistence.NewPostgresRepository(gormDB)
	outboxRepo := persistence.NewOutboxRepository(gormDB)

	// Initialize CQRS Command Handlers
	clock := persistence.SystemClock{}
	ids := persistence.RandomIDGenerator{}

	createHandler := command.NewCreateExerciseHandler(repo, clock, ids)
	updateHandler := command.NewUpdateExerciseHandler(repo, clock)
	submitForApprovalHandler := command.NewSubmitExerciseForApprovalHandler(repo, clock, ids)
	approveHandler := command.NewApproveExerciseHandler(repo, clock, ids)
	archiveHandler := command.NewArchiveExerciseHandler(repo, clock, ids)

	// Initialize CQRS Query Handlers
	getHandler := query.NewGetExerciseHandler(repo)
	searchHandler := query.NewSearchExercisesHandler(repo)
	metadataHandler := query.NewGetCatalogMetadataHandler(repo)

	// Initialize gRPC Handler and Register Service
	grpcHandler := transport.NewExerciseServer(
		createHandler,
		updateHandler,
		submitForApprovalHandler,
		approveHandler,
		archiveHandler,
		getHandler,
		searchHandler,
		metadataHandler,
	)
	exercisesvc.RegisterExerciseServiceServer(deps.GRPCServer, grpcHandler)

	// Start Background Worker for Outbox Pattern & Kafka
	kafkaBrokersStr := os.Getenv("EXERCISE_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	writer, err := deps.KafkaRegistry.GetWriter("exercise", kafkaBrokers)
	if err != nil {
		return nil, fmt.Errorf("get exercise kafka writer: %w", err)
	}

	kafkaPub := kafka.NewPublisher(writer)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 1*time.Second)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in Exercise Outbox background worker: %v", r)
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	// Shutdown callback function
	shutdown := func() {
		log.Println("Shutting down Exercise Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		log.Println("Exercise Bounded Context background workers stopped. Closing Kafka publisher...")
		if err := kafkaPub.Close(); err != nil {
			log.Printf("WARNING: failed to close exercise Kafka publisher: %v", err)
		}
		log.Println("Exercise Bounded Context Kafka publisher closed successfully.")
	}

	log.Println("Exercise Bounded Context initialized successfully.")
	return shutdown, nil
}

func RegisterGateway(
	ctx context.Context,
	mux *runtime.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	err := exercisesvc.RegisterExerciseServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register exercise service gateway handler: %w", err)
	}
	return nil
}
