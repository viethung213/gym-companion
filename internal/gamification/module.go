package gamification

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
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	infraEvent "github.com/viethung213/gym-companion/internal/gamification/infrastructure/event"
	infraKafka "github.com/viethung213/gym-companion/internal/gamification/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/gamification/transport/consumer"
	gamificationGRPC "github.com/viethung213/gym-companion/internal/gamification/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/service/gamificationv1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ModuleDeps contains external dependencies required to boot the Gamification module.
type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

// Initialize wires domain, application, infrastructure, and transport components.
func Initialize(ctx context.Context, deps ModuleDeps) (handler *gamificationGRPC.Handler, shutdown func(), err error) {
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
		return nil, nil, fmt.Errorf("open gorm db for gamification: %w", err)
	}

	// 1. Persistence Layer
	txManager := persistence.NewSQLTransactionManager(gormDB)
	outboxRepo := persistence.NewGormOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewGormOutboxLogRepository(gormDB)
	userXpRepo := persistence.NewGormUserXpRepository(gormDB)
	userStreakRepo := persistence.NewGormUserStreakRepository(gormDB)
	leaderboardRepo := persistence.NewGormLeaderboardRepository(gormDB)
	badgeRepo := persistence.NewGormBadgeRepository(gormDB)

	outboxWriter := infraEvent.NewOutboxWriter(outboxRepo)

	// 2. Application Commands
	processWorkoutXpHandler := command.NewProcessWorkoutXpHandler(userXpRepo, txManager, outboxWriter)
	processNutritionXpHandler := command.NewProcessNutritionXpHandler(userXpRepo, txManager, outboxWriter)
	recordStreakHandler := command.NewRecordStreakActivityHandler(userStreakRepo, txManager, outboxWriter)
	syncUserSnapshotHandler := command.NewSyncUserSnapshotHandler(leaderboardRepo)
	initUserGamificationHandler := command.NewInitUserGamificationHandler(userXpRepo, userStreakRepo, leaderboardRepo, txManager)
	processPersonalRecordHandler := command.NewProcessPersonalRecordHandler(userXpRepo, badgeRepo, txManager, outboxWriter)
	processNutritionActivityHandler := command.NewProcessNutritionActivityHandler(recordStreakHandler, processNutritionXpHandler, txManager)

	// 3. Coordinator
	coordinator := appService.NewActivityIngestionCoordinator(
		processWorkoutXpHandler,
		recordStreakHandler,
		userStreakRepo,
		badgeRepo,
		txManager,
		outboxWriter,
	)

	// 4. Application Queries
	getMyXpHandler := query.NewGetMyXpHandler(userXpRepo)
	getXpHistoryHandler := query.NewGetXpHistoryHandler(userXpRepo)
	getStreakHandler := query.NewGetStreakHandler(userStreakRepo)
	getStreakHistoryHandler := query.NewGetStreakHistoryHandler(userStreakRepo)
	getGlobalLeaderboardHandler := query.NewGetGlobalLeaderboardHandler(leaderboardRepo)
	getMyBadgesHandler := query.NewGetMyBadgesHandler(badgeRepo)

	// 5. Transport gRPC Handler
	grpcHandler := gamificationGRPC.NewGRPCHandler(
		getMyXpHandler,
		getXpHistoryHandler,
		getStreakHandler,
		getStreakHistoryHandler,
		getGlobalLeaderboardHandler,
		getMyBadgesHandler,
	)

	// 6. Kafka & Workers Setup
	var (
		kafkaPub                    *infraKafka.Publisher
		userRegisteredConsumer      *consumer.UserRegisteredConsumer
		workoutEventConsumer        *consumer.WorkoutEventConsumer
		personalRecordConsumer      *consumer.PersonalRecordConsumer
		nutritionEventConsumer      *consumer.NutritionEventConsumer
		userIdentityUpdatedConsumer *consumer.UserIdentityUpdatedConsumer
	)

	if deps.KafkaRegistry != nil {
		kafkaBrokers := os.Getenv("KAFKA_BROKERS")
		if kafkaBrokers == "" {
			kafkaBrokers = "localhost:9092"
		}
		brokers := strings.Split(kafkaBrokers, ",")

		if writer, wErr := deps.KafkaRegistry.GetWriter("gamification.events", brokers); wErr == nil {
			kafkaPub = infraKafka.NewPublisher(writer)
		} else {
			log.Printf("[Gamification] Warning: failed to create Kafka writer: %v", wErr)
		}

		if authReader, aErr := deps.KafkaRegistry.GetReader("gamification-user-registered-group", "auth.events", brokers); aErr == nil {
			userRegisteredConsumer = consumer.NewUserRegisteredConsumer(authReader, initUserGamificationHandler, outboxLogRepo)
		}

		if workoutReader, woErr := deps.KafkaRegistry.GetReader("gamification-workout-group", "workout_execution.events", brokers); woErr == nil {
			workoutEventConsumer = consumer.NewWorkoutEventConsumer(workoutReader, coordinator, outboxLogRepo)
		}

		if prReader, prErr := deps.KafkaRegistry.GetReader("gamification-pr-group", "workout_execution.events", brokers); prErr == nil {
			personalRecordConsumer = consumer.NewPersonalRecordConsumer(prReader, processPersonalRecordHandler, outboxLogRepo)
		}

		if nutritionReader, nErr := deps.KafkaRegistry.GetReader("gamification-nutrition-group", "nutrition.events", brokers); nErr == nil {
			nutritionEventConsumer = consumer.NewNutritionEventConsumer(nutritionReader, processNutritionActivityHandler, outboxLogRepo)
		}

		if profileReader, pErr := deps.KafkaRegistry.GetReader("gamification-profile-group", "profile.events", brokers); pErr == nil {
			userIdentityUpdatedConsumer = consumer.NewUserIdentityUpdatedConsumer(profileReader, syncUserSnapshotHandler, outboxLogRepo)
		}
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	if kafkaPub != nil {
		outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 5*time.Second)
		wg.Add(1)
		go func() {
			defer wg.Done()
			outboxWorker.Start(workerCtx)
		}()
	}

	if userRegisteredConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userRegisteredConsumer.Start(workerCtx)
		}()
	}

	if workoutEventConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workoutEventConsumer.Start(workerCtx)
		}()
	}

	if personalRecordConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			personalRecordConsumer.Start(workerCtx)
		}()
	}

	if nutritionEventConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nutritionEventConsumer.Start(workerCtx)
		}()
	}

	if userIdentityUpdatedConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userIdentityUpdatedConsumer.Start(workerCtx)
		}()
	}

	shutdown = func() {
		log.Println("[Gamification] Shutting down background workers...")
		cancelWorkers()
		wg.Wait()
		log.Println("[Gamification] Background workers shutdown complete.")
	}

	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the Gamification service on the provided HTTP multiplexer.
func RegisterConnectHandler(mux *http.ServeMux, handler *gamificationGRPC.Handler, opts ...connect.HandlerOption) {
	path, connectHandler := gamificationv1serviceconnect.NewGamificationServiceHandler(handler, opts...)
	mux.Handle(path, connectHandler)
}
