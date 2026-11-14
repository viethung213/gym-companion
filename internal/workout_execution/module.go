package workoutexecution

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	workoutConfig "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/config"
	workoutEvent "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/event"
	workoutKafka "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
	workoutRule "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
	workoutStorage "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/storage"
	workoutSync "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/sync"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
	workoutConsumer "github.com/viethung213/gym-companion/internal/workout_execution/transport/consumer"
	workoutGRPC "github.com/viethung213/gym-companion/internal/workout_execution/transport/grpc"
	"github.com/viethung213/gym-companion/internal/workout_execution/transport/webtransport"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ModuleDeps contains external dependencies required to boot the Workout Execution module.
type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
	RedisClient   *redis.Client
}

// Initialize wires dependencies, registers gRPC services, and starts background workers.
func Initialize(ctx context.Context, deps ModuleDeps) (*workoutGRPC.GRPCHandler, func(), error) {

	gormLogger := logger.Default.LogMode(logger.Warn)

	gormDB, err := database.OpenGorm(deps.DB, &gorm.Config{
		Logger:                 gormLogger,
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

	eventRecorder := workoutEvent.NewOutboxWriter(outboxRepo)

	loadGuard := service.NewTrainingLoadGuard(sessionRepo)
	storageProvider := workoutStorage.NewS3StorageProviderFromEnv()

	// Initialize Command Handlers
	startSessionHandler := command.NewStartWorkoutSessionHandler(sessionRepo, nil, eventRecorder, txManager)
	startScheduledSessionHandler := command.NewStartScheduledWorkoutSessionHandler(sessionRepo, eventRecorder, txManager)
	logSetHandler := command.NewLogWorkoutSetHandler(sessionRepo, eventRecorder, txManager)
	completeSessionHandler := command.NewCompleteWorkoutSessionHandler(sessionRepo, loadGuard, nil, eventRecorder, txManager)
	shareSessionHandler := command.NewShareWorkoutSessionHandler(sessionRepo, motionRepo, eventRecorder, txManager)
	abortSessionHandler := command.NewAbortWorkoutSessionHandler(sessionRepo, eventRecorder, txManager)
	syncLogsHandler := command.NewSyncWorkoutLogsHandler(sessionRepo, eventRecorder, txManager)
	prProcessHandler := command.NewProcessCompletedSessionForPRHandler(sessionRepo, prRepo, eventRecorder, txManager)
	updateMotionSpecHandler := command.NewUpdateMotionSpecificationHandler(motionRepo, eventRecorder, txManager)
	patchMotionSpecAssetHandler := command.NewPatchMotionSpecificationAssetHandler(motionRepo, storageProvider, eventRecorder, txManager)
	deleteMotionSpecHandler := command.NewDeleteMotionSpecificationHandler(motionRepo)

	// Initialize Query Handlers
	getMotionSpecQuery := query.NewGetMotionSpecificationQueryHandler(motionRepo)
	getPRsQuery := query.NewGetPersonalRecordsQueryHandler(prRepo)
	getErrorsQuery := query.NewGetWorkoutSessionErrorsQueryHandler(sessionRepo)
	getHistoryQuery := query.NewGetWorkoutHistoryQueryHandler(sessionRepo)
	listMotionSpecsQuery := query.NewListMotionSpecificationsQueryHandler(motionRepo)
	searchMotionSpecsQuery := query.NewSearchMotionSpecificationsQueryHandler(motionRepo)
	getMotionSpecStatsQuery := query.NewGetMotionSpecificationStatsQueryHandler(motionRepo)
	getPresignedUploadURLQuery := query.NewGetPresignedUploadURLQueryHandler(storageProvider)

	// Solo Match streaming dependencies (Redis-backed distributed live sync via ModuleDeps)
	ruleProvider := workoutRule.NewCachedRuleProvider(motionRepo, storageProvider)

	var liveSync port.SoloMatchLiveSyncPort
	if deps.RedisClient != nil {
		liveSync = workoutSync.NewRedisLiveSyncHub(deps.RedisClient, 2*time.Hour)
	}
	soloTrackingService := command.NewSoloTrackingService(
		ruleProvider,
		service.NewDefaultRuleEvaluator(),
		liveSync,
		eventRecorder,
		sessionRepo,
		txManager,
	)

	// Initialize gRPC Transport
	grpcHandler := workoutGRPC.NewGRPCHandler(
		startSessionHandler,
		startScheduledSessionHandler,
		logSetHandler,
		completeSessionHandler,
		shareSessionHandler,
		abortSessionHandler,
		syncLogsHandler,
		getMotionSpecQuery,
		getPRsQuery,
		getErrorsQuery,
		getHistoryQuery,
		updateMotionSpecHandler,
		deleteMotionSpecHandler,
		listMotionSpecsQuery,
		searchMotionSpecsQuery,
		getMotionSpecStatsQuery,
		getPresignedUploadURLQuery,
		patchMotionSpecAssetHandler,
		soloTrackingService,
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
	timeoutWorker := worker.NewSessionTimeoutWorker(sessionRepo, eventRecorder, txManager, 5*time.Minute)
	criticalWorker := worker.NewCriticalInactivityWorker(sessionRepo, eventRecorder, txManager, 1*time.Minute, 5*time.Minute)
	soloBotManager := worker.NewSoloBotManager(prRepo, liveSync, eventRecorder)
	exerciseCreatedConsumer := workoutConsumer.NewExerciseCreatedConsumer(motionRepo, outboxLogRepo)
	prConsumer := workoutConsumer.NewPREventConsumer(prProcessHandler)
	soloMatchStartedConsumer := workoutConsumer.NewSoloMatchStartedConsumer(soloBotManager, outboxLogRepo)
	soloMatchCancelledConsumer := workoutConsumer.NewSoloMatchCancelledConsumer(liveSync, soloBotManager, outboxLogRepo)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in WorkoutExecution Outbox worker", slog.Any("panic", r))
			}
		}()
		outboxWorker.Start(workerCtx)
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in WorkoutExecution Timeout worker", slog.Any("panic", r))
			}
		}()
		timeoutWorker.Start(workerCtx)
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in WorkoutExecution Critical Inactivity worker", slog.Any("panic", r))
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
						slog.Error("PANIC RECOVERED in WorkoutExecution ExerciseCreated consumer worker", slog.Any("panic", r))
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
							slog.Error("WorkoutExecution exercise reader error", slog.String("error", err.Error()))
							time.Sleep(1 * time.Second)
							continue
						}

						func() {
							defer func() {
								if r := recover(); r != nil {
									slog.Error("PANIC RECOVERED in ExerciseCreated event handling", slog.Any("panic", r))
								}
							}()
							if err := exerciseCreatedConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								slog.Error("WorkoutExecution failed to process ExerciseCreated event", slog.String("error", err.Error()))
							}
						}()
					}
				}
			}()
		} else if err != nil {
			slog.Warn("Failed to obtain exercise events reader", slog.String("error", err.Error()))
		}

		// Start WorkoutSessionCompleted event consumer worker (listening to topic 'workout_execution.events')
		sessionCompletedReader, err := deps.KafkaRegistry.GetReader("workout_execution_pr_consumer", "workout_execution.events", kafkaBrokers)
		if err == nil && sessionCompletedReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("PANIC RECOVERED in WorkoutExecution PREventConsumer worker", slog.Any("panic", r))
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
							slog.Error("WorkoutExecution PR consumer reader error", slog.String("error", err.Error()))
							time.Sleep(1 * time.Second)
							continue
						}

						func() {
							defer func() {
								if r := recover(); r != nil {
									slog.Error("PANIC RECOVERED in PREventConsumer event handling", slog.Any("panic", r))
								}
							}()
							if err := prConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								slog.Error("WorkoutExecution failed to process WorkoutSessionCompleted event for PR", slog.String("error", err.Error()))
							}
						}()
					}
				}
			}()
		} else if err != nil {
			slog.Warn("Failed to obtain PR consumer reader", slog.String("error", err.Error()))
		}

		// Start SoloMatchStarted event consumer worker (listening to topic 'solo_match.events')
		soloMatchReader, err := deps.KafkaRegistry.GetReader("workout_execution_solo_match_consumer", "solo_match.events", kafkaBrokers)
		if err == nil && soloMatchReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("PANIC RECOVERED in WorkoutExecution SoloMatchStarted consumer worker", slog.Any("panic", r))
					}
					_ = soloMatchReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := soloMatchReader.ReadMessage(workerCtx)
						if err != nil {
							if errors.Is(err, context.Canceled) {
								return
							}
							slog.Error("WorkoutExecution solo match reader error", slog.String("error", err.Error()))
							time.Sleep(1 * time.Second)
							continue
						}

						func() {
							defer func() {
								if r := recover(); r != nil {
									slog.Error("PANIC RECOVERED in SoloMatch event handling", slog.Any("panic", r))
								}
							}()
							if err := soloMatchStartedConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								slog.Error("WorkoutExecution failed to process SoloMatchStarted event", slog.String("error", err.Error()))
							}
							if err := soloMatchCancelledConsumer.HandleMessage(workerCtx, msg.Value); err != nil {
								slog.Error("WorkoutExecution failed to process SoloMatchCancelled event", slog.String("error", err.Error()))
							}
						}()
					}
				}
			}()
		} else if err != nil {
			slog.Warn("Failed to obtain solo match reader", slog.String("error", err.Error()))
		}
	}

	shutdown := func() {
		slog.Info("Shutting down Workout Execution Bounded Context background workers")
		cancelWorkers()
		soloBotManager.StopAll()
		wg.Wait()
		slog.Info("Workout Execution Bounded Context background workers stopped successfully")
	}

	slog.Info("Workout Execution Bounded Context initialized successfully")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler registers the ConnectRPC handlers for the Workout Execution module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	grpcHandler *workoutGRPC.GRPCHandler,
) []string {
	connectHandler := workoutGRPC.NewConnectWorkoutExecutionHandler(grpcHandler)
	workoutexecutionv1serviceconnect.RegisterWorkoutExecutionServiceHandler(server, connectHandler)
	workoutexecutionv1serviceconnect.RegisterAdminWorkoutExecutionServiceHandler(server, connectHandler)
	return []string{
		"/" + workoutexecutionv1serviceconnect.WorkoutExecutionServiceName + "/",
		"/" + workoutexecutionv1serviceconnect.AdminWorkoutExecutionServiceName + "/",
	}
}

// RegisterWebTransport mounts the WebTransport config endpoint & WebSocket fallback on mux,
// starts the HTTP/3 QUIC WebTransport server in the background, and returns a shutdown function and registered HTTP prefixes.
func RegisterWebTransport(
	mux *http.ServeMux,
	grpcHandler *workoutGRPC.GRPCHandler,
	validator middleware.TokenValidator,
	wtAddr string,
	internalSecret string,
) (func(), []string, error) {
	if grpcHandler == nil || grpcHandler.SoloTrackingService() == nil {
		return func() {}, nil, fmt.Errorf("solo tracking service is not configured")
	}

	if validator == nil {
		return func() {}, nil, errors.New("webtransport: token validator is required")
	}
	wtServer, err := webtransport.New(webtransport.Config{
		Addr:           wtAddr,
		InternalSecret: internalSecret,
		Validator:      validator,
		Service:        grpcHandler.SoloTrackingService(),
		IdleTimeout:    30 * time.Second,
	})
	if err != nil {
		return func() {}, nil, fmt.Errorf("create webtransport server: %w", err)
	}

	if err := wtServer.Start(); err != nil {
		return func() {}, nil, fmt.Errorf("start webtransport server on %s: %w", wtAddr, err)
	}

	// Mount HTTP config endpoint and WebSocket fallback on the shared HTTP mux
	mux.Handle(webtransport.ConfigPath, wtServer.ConfigHandler())
	mux.Handle(webtransport.SoloWebSocketPath, wtServer.SoloWebSocketHandler())

	slog.Info("WebTransport HTTP/3 server listening", slog.String("addr", wtAddr))
	slog.Info("WebTransport mounted", slog.String("config_path", webtransport.ConfigPath), slog.String("ws_path", webtransport.SoloWebSocketPath))

	endpoints := []string{
		webtransport.ConfigPath,
		webtransport.SoloWebSocketPath,
	}

	return func() {
		slog.Info("Shutting down WebTransport server")
		_ = wtServer.Close()
		slog.Info("WebTransport server stopped")
	}, endpoints, nil
}
