package workoutexecution

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	workoutConfig "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/config"
	workoutEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
	workoutKafka "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/storage"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
	workoutConsumer "github.com/viethung213/gym-companion/internal/workout_execution/transport/consumer"
	workoutGRPC "github.com/viethung213/gym-companion/internal/workout_execution/transport/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ModuleDeps contains external dependencies required to boot the Workout Execution module.
type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

// Initialize wires dependencies, registers gRPC services, and starts background workers.
func Initialize(ctx context.Context, deps ModuleDeps) (*workoutGRPC.GRPCHandler, func(), error) {

	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		PrepareStmt:            false,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// Initialize Repositories & Storage & Transaction Manager
	txManager := persistence.NewSQLTransactionManager(gormDB)
	sessionRepo := persistence.NewPostgresWorkoutSessionRepository(gormDB)
	prRepo := persistence.NewPostgresPersonalRecordRepository(gormDB)
	motionRepo := persistence.NewPostgresMotionSpecificationRepository(gormDB)
	outboxRepo := persistence.NewPostgresOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewPostgresOutboxLogRepository(gormDB)

	outboxWriter := workoutEvent.NewOutboxWriter(outboxRepo)

	loadGuard := service.NewTrainingLoadGuard(sessionRepo)
	storageProvider := storage.NewS3StorageProviderFromEnv()

	// Initialize Command Handlers
	startSessionHandler := command.NewStartWorkoutSessionHandler(sessionRepo, nil, outboxWriter, txManager)
	startScheduledSessionHandler := command.NewStartScheduledWorkoutSessionHandler(sessionRepo, outboxWriter, txManager)
	logSetHandler := command.NewLogWorkoutSetHandler(sessionRepo, outboxWriter, txManager)
	completeSessionHandler := command.NewCompleteWorkoutSessionHandler(sessionRepo, loadGuard, nil, outboxWriter, txManager)
	abortSessionHandler := command.NewAbortWorkoutSessionHandler(sessionRepo, outboxWriter, txManager)
	syncLogsHandler := command.NewSyncWorkoutLogsHandler(sessionRepo, outboxWriter, txManager)
	prProcessHandler := command.NewProcessCompletedSessionForPRHandler(sessionRepo, prRepo, outboxWriter, txManager)
	updateMotionSpecHandler := command.NewUpdateMotionSpecificationHandler(motionRepo, outboxWriter, txManager)
	patchMotionSpecAssetHandler := command.NewPatchMotionSpecificationAssetHandler(motionRepo, storageProvider, outboxWriter, txManager)
	deleteMotionSpecHandler := command.NewDeleteMotionSpecificationHandler(motionRepo)

	// Initialize Query Handlers
	getMotionSpecQuery := query.NewGetMotionSpecificationQueryHandler(motionRepo)
	getPRsQuery := query.NewGetPersonalRecordsQueryHandler(prRepo)
	getErrorsQuery := query.NewGetWorkoutSessionErrorsQueryHandler(sessionRepo)
	getHistoryQuery := query.NewGetWorkoutHistoryQueryHandler(sessionRepo)
	listMotionSpecsQuery := query.NewListMotionSpecificationsQueryHandler(motionRepo)
	getPresignedUploadURLQuery := query.NewGetPresignedUploadURLQueryHandler(storageProvider)

	// Initialize gRPC Transport
	grpcHandler := workoutGRPC.NewGRPCHandler(
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
		updateMotionSpecHandler,
		deleteMotionSpecHandler,
		listMotionSpecsQuery,
		getPresignedUploadURLQuery,
		patchMotionSpecAssetHandler,
	)

	// Kafka Setup
	appCfg := workoutConfig.LoadConfig()
	kafkaBrokers := strings.Split(appCfg.KafkaBrokers, ",")

	var kafkaPub *workoutKafka.Publisher
	if deps.KafkaRegistry != nil && len(kafkaBrokers) > 0 {
		writer, err := deps.KafkaRegistry.GetWriter("workout_execution.events", kafkaBrokers)
		if err == nil && writer != nil {
			kafkaPub = workoutKafka.NewPublisher(writer)
		}
	}

	// Initialize Workers
	outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 2*time.Second)
	timeoutWorker := worker.NewSessionTimeoutWorker(sessionRepo, outboxWriter, txManager, 5*time.Minute)
	criticalWorker := worker.NewCriticalInactivityWorker(sessionRepo, outboxWriter, txManager, 1*time.Minute, 5*time.Minute)
	exerciseCreatedConsumer := workoutConsumer.NewExerciseCreatedConsumer(motionRepo, outboxLogRepo)

	prConsumer := workoutConsumer.NewPREventConsumer(prProcessHandler)

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

	// Start ExerciseCreated event consumer worker (listening to topic 'exercise.events')
	if deps.KafkaRegistry != nil && len(kafkaBrokers) > 0 {
		exerciseReader, err := deps.KafkaRegistry.GetReader("workout_execution_exercise_consumer", "exercise.events", kafkaBrokers)
		if err == nil && exerciseReader != nil {

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in WorkoutExecution ExerciseCreated consumer worker: %v", r)
					}
					_ = exerciseReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := exerciseReader.ReadMessage(workerCtx)
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
									log.Printf("PANIC RECOVERED in ExerciseCreated event handling: %v", r)
								}
							}()
							if err := exerciseCreatedConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								log.Printf("WorkoutExecution failed to process ExerciseCreated event: %v", err)
							}
						}()
					}
				}
			}()
		}

		// Start WorkoutSessionCompleted event consumer worker (listening to topic 'workout_execution.events')
		sessionCompletedReader, err := deps.KafkaRegistry.GetReader("workout_execution_pr_consumer", "workout_execution.events", kafkaBrokers)
		if err == nil && sessionCompletedReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in WorkoutExecution PREventConsumer worker: %v", r)
					}
					_ = sessionCompletedReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := sessionCompletedReader.ReadMessage(workerCtx)
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
									log.Printf("PANIC RECOVERED in PREventConsumer event handling: %v", r)
								}
							}()
							if err := prConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								log.Printf("WorkoutExecution failed to process WorkoutSessionCompleted event for PR: %v", err)
							}
						}()
					}
				}
			}()
		}
	}

	shutdown := func() {
		log.Println("Shutting down Workout Execution Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		log.Println("Workout Execution Bounded Context background workers stopped successfully.")
	}

	log.Println("Workout Execution Bounded Context initialized successfully.")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handlers for the Workout Execution module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *workoutGRPC.GRPCHandler,
	opts ...connect.HandlerOption,
) {
	connectHandler := workoutGRPC.NewConnectWorkoutExecutionHandler(grpcHandler)
	path, handler := workoutexecutionv1serviceconnect.NewWorkoutExecutionServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)

	adminPath, adminHandler := workoutexecutionv1serviceconnect.NewAdminWorkoutExecutionServiceHandler(connectHandler, opts...)
	mux.Handle(adminPath, adminHandler)
}
