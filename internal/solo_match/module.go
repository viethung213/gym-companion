package solomatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/service/solomatchv1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/application/query"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	infraEvent "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/event"
	exerciseclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/exercise_client"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/matchmaker"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/publisher"
	workoutclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/workout_client"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ModuleDeps contains external dependencies required to boot the Solo Match module.
type ModuleDeps struct {
	DB               *sql.DB
	RedisClient      *redis.Client
	KafkaRegistry    *sharedKafka.Registry
	ExerciseProvider port.ExerciseProvider
	UserPRProvider   port.UserPRProvider
}

// Initialize wires dependencies, registers services, and starts background workers.
func Initialize(ctx context.Context, deps ModuleDeps) (*grpc.Handler, func(), error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		Logger:                 gormLogger,
		PrepareStmt:            false,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm for solo_match: %w", err)
	}

	// Redis Client Dependency Injection
	if deps.RedisClient == nil {
		return nil, nil, errors.New("solo_match requires a non-nil RedisClient in ModuleDeps")
	}
	redisClient := deps.RedisClient

	// Repositories & Transaction Manager
	txManager := persistence.NewSQLTransactionManager(gormDB)
	matchRepo := persistence.NewGORMSoloMatchRepository(gormDB)
	snapshotRepo := persistence.NewGORMUserSnapshotRepository(gormDB)
	outboxRepo := persistence.NewGORMOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewGORMOutboxLogRepository(gormDB)
	botRepo := persistence.NewGORMBotRepository(gormDB)

	// Event Publisher Adapter
	eventPub := infraEvent.NewOutboxWriter(outboxRepo)

	// Domain Services
	matchResolver := service.NewMatchResolver()
	botCalc := service.NewBotBenchmarkCalculator()

	exerciseProvider := deps.ExerciseProvider
	if exerciseProvider == nil {
		exerciseProvider = exerciseclient.NewStubExerciseClient()
	}

	prProvider := deps.UserPRProvider
	if prProvider == nil {
		prProvider = workoutclient.NewStubWorkoutClient()
	}

	// Matchmaker Queue Port (Pure Redis In-Memory)
	queuePort := matchmaker.NewRedisMatchmakerAdapter(
		redisClient,
		matchRepo,
		botCalc,
		botRepo,
		exerciseProvider,
		prProvider,
		250*time.Millisecond,
		5*time.Minute,
	)

	// Application Command & Query Handlers
	joinQueueHandler := command.NewJoinMatchmakingQueueHandler(queuePort, matchRepo)
	cancelQueueHandler := command.NewCancelMatchmakingHandler(queuePort)
	recordResultHandler := command.NewRecordParticipantResultHandler(matchRepo, eventPub, txManager, matchResolver, botRepo)
	recordAbortHandler := command.NewRecordParticipantAbortHandler(matchRepo, eventPub, txManager, botRepo)
	timeoutHandler := command.NewHandleMatchTimeoutHandler(matchRepo, eventPub, txManager, botRepo)
	syncSnapshotHandler := command.NewSyncUserSnapshotHandler(snapshotRepo)

	getMatchHandler := query.NewGetSoloMatchByIDHandler(matchRepo, snapshotRepo)
	getHistoryHandler := query.NewGetUserMatchHistoryHandler(matchRepo, snapshotRepo)

	// Transport Handlers
	grpcHandler := grpc.NewHandler(
		joinQueueHandler,
		cancelQueueHandler,
		getMatchHandler,
		getHistoryHandler,
		snapshotRepo,
	)

	// Kafka Setup
	kafkaBrokersStr := os.Getenv("KAFKA_BROKERS")
	var kafkaBrokers []string
	if kafkaBrokersStr != "" {
		for _, b := range strings.Split(kafkaBrokersStr, ",") {
			if trimmed := strings.TrimSpace(b); trimmed != "" {
				kafkaBrokers = append(kafkaBrokers, trimmed)
			}
		}
	}

	var kafkaWriter *kafka.Writer
	if deps.KafkaRegistry != nil && len(kafkaBrokers) > 0 {
		writer, wErr := deps.KafkaRegistry.GetWriter("solo_match.events", kafkaBrokers)
		if wErr == nil && writer != nil {
			kafkaWriter = writer
		} else {
			log.Printf("[SoloMatch] Warning: failed to obtain kafka writer: %v", wErr)
		}
	}

	// Workers & Consumers Lifecycle
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	// 1. Outbox Publisher Worker
	outboxPub := publisher.NewOutboxPublisher(outboxRepo, kafkaWriter, 2*time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in SoloMatch Outbox Publisher: %v", r)
			}
		}()
		outboxPub.Start(workerCtx)
	}()

	// 2. Fallback Match Timeout Worker (Safety net every 15 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in SoloMatch Match Timeout Worker: %v", r)
			}
		}()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC in SoloMatch timeout check tick: %v", r)
						}
					}()
					if _, tErr := timeoutHandler.Handle(workerCtx, command.HandleMatchTimeoutCommand{}); tErr != nil {
						log.Printf("[SoloMatch] Timeout worker sweep error: %v", tErr)
					}
				}()
			}
		}
	}()

	// 3. Kafka Consumers (Workout & User events)
	if deps.KafkaRegistry != nil && len(kafkaBrokers) > 0 {
		workoutCompletedConsumer := consumer.NewWorkoutCompletedConsumer(recordResultHandler, outboxLogRepo)
		workoutAbortedConsumer := consumer.NewWorkoutAbortedConsumer(recordAbortHandler, outboxLogRepo)
		userRegisteredConsumer := consumer.NewUserRegisteredConsumer(syncSnapshotHandler, outboxLogRepo)
		userIdentityUpdatedConsumer := consumer.NewUserIdentityUpdatedConsumer(syncSnapshotHandler, outboxLogRepo)

		// Consumer 3A: Workout Execution Events
		workoutReader, rErr := deps.KafkaRegistry.GetReader("solo_match_workout_consumer", "workout_execution.events", kafkaBrokers)
		if rErr == nil && workoutReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in SoloMatch Workout Events Consumer: %v", r)
					}
					_ = workoutReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := workoutReader.ReadMessage(workerCtx)
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
									log.Printf("PANIC in processing workout execution event: %v", r)
								}
							}()
							_ = workoutCompletedConsumer.HandleMessage(workerCtx, msg.Value)
							_ = workoutAbortedConsumer.HandleMessage(workerCtx, msg.Value)
						}()
					}
				}
			}()
		}

		// Consumer 3B: Auth Events (User Registered)
		authReader, aErr := deps.KafkaRegistry.GetReader("solo_match_user_consumer", "auth.events", kafkaBrokers)
		if aErr == nil && authReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in SoloMatch Auth Events Consumer: %v", r)
					}
					_ = authReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := authReader.ReadMessage(workerCtx)
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
									log.Printf("PANIC in processing auth event: %v", r)
								}
							}()
							_ = userRegisteredConsumer.HandleMessage(workerCtx, msg.Value)
						}()
					}
				}
			}()
		}

		// Consumer 3C: Profile Events (User Identity Updated)
		profileReader, pErr := deps.KafkaRegistry.GetReader("solo_match_profile_consumer", "profile.events", kafkaBrokers)
		if pErr == nil && profileReader != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in SoloMatch Profile Events Consumer: %v", r)
					}
					_ = profileReader.Close()
				}()

				for {
					select {
					case <-workerCtx.Done():
						return
					default:
						msg, err := profileReader.ReadMessage(workerCtx)
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
									log.Printf("PANIC in processing profile event: %v", r)
								}
							}()
							_ = userIdentityUpdatedConsumer.HandleMessage(workerCtx, msg.Value)
						}()
					}
				}
			}()
		}
	}

	shutdown := func() {
		log.Println("Shutting down Solo Match Bounded Context background workers...")
		outboxPub.Stop()
		cancelWorkers()
		wg.Wait()
		log.Println("Solo Match Bounded Context background workers stopped successfully.")
	}

	log.Println("Solo Match Bounded Context initialized successfully.")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handlers for the Solo Match module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *grpc.Handler,
	opts ...connect.HandlerOption,
) {
	connectHandler := grpc.NewConnectSoloMatchHandler(grpcHandler)
	path, handler := solomatchv1serviceconnect.NewSoloMatchServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
