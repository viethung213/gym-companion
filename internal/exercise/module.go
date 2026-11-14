package exercise

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	submitForApprovalHandler := command.NewSubmitExerciseForApprovalHandler(repo, clock, ids)
	approveHandler := command.NewApproveExerciseHandler(repo, clock, ids)
	archiveHandler := command.NewArchiveExerciseHandler(repo, clock, ids)
	restoreHandler := command.NewRestoreExerciseHandler(repo, clock, ids)

	// Initialize CQRS Query Handlers
	getHandler := query.NewGetExerciseHandler(repo)
	getRandomHandler := query.NewGetRandomExerciseHandler(repo)
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
		restoreHandler,
		getHandler,
		getRandomHandler,
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
	cfg := config.LoadConfig()
	kafkaBrokers := strings.Split(cfg.KafkaBrokers, ",")

	writer, err := deps.KafkaRegistry.GetWriter("exercise.events", kafkaBrokers)
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

	// Start MotionSpec event consumer worker (listening to topic 'workout_execution.events')
	if deps.KafkaRegistry != nil && len(kafkaBrokers) > 0 {
		motionSpecReader, err := deps.KafkaRegistry.GetReader("exercise_motion_spec_consumer", "workout_execution.events", kafkaBrokers)
		if err == nil && motionSpecReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("PANIC RECOVERED in Exercise MotionSpec consumer worker", slog.Any("panic", r))
					}
					_ = motionSpecReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := motionSpecReader.ReadMessage(workerCtx)
						if err != nil {
							if errors.Is(err, context.Canceled) {
								return
							}
							time.Sleep(1 * time.Second)
							continue
						}

						func() {
							defer func() {
								if r := recover(); r != nil {
									slog.Error("PANIC RECOVERED in Exercise MotionSpec event handling", slog.Any("panic", r))
								}
							}()
							if err := motionSpecConsumer.ConsumeMotionSpecReady(workerCtx, msg.Value); err != nil {
								slog.Error("Exercise failed to process MotionSpec event", slog.String("error", err.Error()))
							}
						}()
					}
				}
			}()
		}
	}

	// Shutdown callback function
	shutdown := func() {
		slog.Info("Shutting down Exercise Bounded Context background workers...")
		cancelWorkers()
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
