package exercise

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/persistence"
	exerciseGRPC "github.com/viethung213/gym-companion/internal/exercise/transport/grpc"
	"github.com/viethung213/gym-companion/internal/exercise/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*exerciseGRPC.ExerciseServer, func(), error) {

	// Initialize GORM DB wrapper over sql.DB
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		PrepareStmt:            false,
		SkipDefaultTransaction: true,
	})
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
	submitForApprovalHandler := command.NewSubmitExerciseForApprovalHandler(repo, clock, ids)
	approveHandler := command.NewApproveExerciseHandler(repo, clock, ids)
	archiveHandler := command.NewArchiveExerciseHandler(repo, clock, ids)

	// Initialize CQRS Query Handlers
	getHandler := query.NewGetExerciseHandler(repo)
	searchHandler := query.NewSearchExercisesHandler(repo)
	metadataHandler := query.NewGetCatalogMetadataHandler(repo)

	// Catalog CRUD handlers
	createBodyPartHandler := command.NewCreateBodyPartHandler(repo, ids)
	updateBodyPartHandler := command.NewUpdateBodyPartHandler(repo)
	deleteBodyPartHandler := command.NewDeleteBodyPartHandler(repo)
	getBodyPartHandler := query.NewGetBodyPartHandler(repo)
	listBodyPartsHandler := query.NewListBodyPartsHandler(repo)

	createEquipmentHandler := command.NewCreateEquipmentHandler(repo, ids)
	updateEquipmentHandler := command.NewUpdateEquipmentHandler(repo)
	deleteEquipmentHandler := command.NewDeleteEquipmentHandler(repo)
	getEquipmentHandler := query.NewGetEquipmentHandler(repo)
	listEquipmentsHandler := query.NewListEquipmentsHandler(repo)

	createMuscleHandler := command.NewCreateMuscleHandler(repo, ids)
	updateMuscleHandler := command.NewUpdateMuscleHandler(repo)
	deleteMuscleHandler := command.NewDeleteMuscleHandler(repo)
	getMuscleHandler := query.NewGetMuscleHandler(repo)
	listMusclesHandler := query.NewListMusclesHandler(repo)

	createTagHandler := command.NewCreateTagHandler(repo, ids)
	updateTagHandler := command.NewUpdateTagHandler(repo)
	deleteTagHandler := command.NewDeleteTagHandler(repo)
	getTagHandler := query.NewGetTagHandler(repo)
	listTagsHandler := query.NewListTagsHandler(repo)

	// Initialize gRPC Handler and Register Service
	grpcHandler := exerciseGRPC.NewExerciseServer(
		createHandler,
		updateHandler,
		submitForApprovalHandler,
		approveHandler,
		archiveHandler,
		getHandler,
		searchHandler,
		metadataHandler,
		createBodyPartHandler,
		updateBodyPartHandler,
		deleteBodyPartHandler,
		getBodyPartHandler,
		listBodyPartsHandler,
		createEquipmentHandler,
		updateEquipmentHandler,
		deleteEquipmentHandler,
		getEquipmentHandler,
		listEquipmentsHandler,
		createMuscleHandler,
		updateMuscleHandler,
		deleteMuscleHandler,
		getMuscleHandler,
		listMusclesHandler,
		createTagHandler,
		updateTagHandler,
		deleteTagHandler,
		getTagHandler,
		listTagsHandler,
	)

	// Start Background Worker for Outbox Pattern & Kafka
	kafkaBrokersStr := os.Getenv("EXERCISE_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	writer, err := deps.KafkaRegistry.GetWriter("exercise.events", kafkaBrokers)
	if err != nil {
		return nil, nil, fmt.Errorf("get exercise kafka writer: %w", err)
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
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for the Exercise module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcServer *exerciseGRPC.ExerciseServer,
	opts ...connect.HandlerOption,
) {
	connectHandler := exerciseGRPC.NewConnectExerciseHandler(grpcServer)
	path, handler := exercisev1serviceconnect.NewExerciseServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
