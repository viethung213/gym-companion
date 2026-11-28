package gamification

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
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
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	sharedRedis "github.com/viethung213/gym-companion/internal/shared/redis"
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
	gormLogger := logger.Default.LogMode(logger.Warn)

	gormDB, err := database.OpenGorm(deps.DB, &gorm.Config{
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
	userSnapshotRepo := persistence.NewGormUserSnapshotRepository(gormDB)
	badgeRepo := persistence.NewGormBadgeRepository(gormDB)
	userWalletRepo := persistence.NewGormUserWalletRepository(gormDB)

	outboxWriter := infraEvent.NewOutboxWriter(outboxRepo)

	leaderboard := newLeaderboardReadModel(
		leaderboardRepo,
		userSnapshotRepo,
		connectLeaderboardRedis(),
		func() *kafka.Reader { return leaderboardEventsReader(deps.KafkaRegistry) },
		leaderboardReconcileInterval(),
	)

	// 2. Application Commands
	awardCoinsHandler := command.NewAwardCoinsHandler(userWalletRepo, txManager, outboxWriter)
	spendCoinsHandler := command.NewSpendCoinsHandler(userWalletRepo, txManager, outboxWriter)
	adminAdjustCoinsHandler := command.NewAdminAdjustCoinsHandler(userWalletRepo, txManager, outboxWriter)
	evaluateBadgesHandler := command.NewEvaluateBadgesHandler(badgeRepo, txManager, outboxWriter)

	processWorkoutXpHandler := command.NewProcessWorkoutXpHandler(userXpRepo, txManager, outboxWriter)
	processNutritionXpHandler := command.NewProcessNutritionXpHandler(userXpRepo, txManager, outboxWriter)
	recordStreakHandler := command.NewRecordStreakActivityHandler(userStreakRepo, txManager, outboxWriter)
	syncUserSnapshotHandler := command.NewSyncUserSnapshotHandler(leaderboard.snapshots)
	initUserGamificationHandler := command.NewInitUserGamificationHandler(
		userXpRepo, userStreakRepo, leaderboard.snapshots, txManager,
	).
		WithWalletRepository(userWalletRepo).
		WithLeaderboardProjector(leaderboard.projector)
	processPersonalRecordHandler := command.NewProcessPersonalRecordHandler(userXpRepo, txManager, outboxWriter)

	// 3. Coordinator
	coordinator := appService.NewActivityIngestionCoordinator(
		processWorkoutXpHandler,
		recordStreakHandler,
		badgeRepo,
		txManager,
		outboxWriter,
	).WithBadgeHandler(evaluateBadgesHandler).
		WithWalletRewarder(awardCoinsHandler).
		WithNutritionAndPRHandlers(processNutritionXpHandler, processPersonalRecordHandler).
		WithLeaderboardProjector(leaderboard.projector)

	// 4. Application Queries
	getMyXpHandler := query.NewGetMyXpHandler(userXpRepo)
	getXpHistoryHandler := query.NewGetXpHistoryHandler(userXpRepo)
	getStreakHandler := query.NewGetStreakHandler(userStreakRepo)
	getStreakHistoryHandler := query.NewGetStreakHistoryHandler(userStreakRepo)
	getGlobalLeaderboardHandler := query.NewGetGlobalLeaderboardHandler(leaderboard.repository)
	getMyBadgesHandler := query.NewGetMyBadgesHandler(badgeRepo)
	getWalletHandler := query.NewGetWalletHandler(userWalletRepo)
	listTransactionsHandler := query.NewListTransactionsHandler(userWalletRepo)

	// 5. Transport gRPC Handler
	grpcHandler := gamificationGRPC.NewGRPCHandler(
		getMyXpHandler,
		getXpHistoryHandler,
		getStreakHandler,
		getStreakHistoryHandler,
		getGlobalLeaderboardHandler,
		getMyBadgesHandler,
	).WithWalletHandlers(
		getWalletHandler,
		listTransactionsHandler,
		spendCoinsHandler,
		adminAdjustCoinsHandler,
	)

	// 6. Kafka & Workers Setup
	var (
		kafkaPub    *infraKafka.Publisher
		consumerMgr *consumer.ConsumerManager
	)

	if deps.KafkaRegistry != nil {
		if writer, wErr := deps.KafkaRegistry.GetWriter("gamification.events"); wErr == nil {
			kafkaPub = infraKafka.NewPublisher(writer)
		} else {
			slog.Warn("Failed to create Kafka writer", slog.String("module", "gamification"), slog.Any("error", wErr))
		}

		consumerMgr = consumer.NewConsumerManager(
			deps.KafkaRegistry,
			outboxLogRepo,
			initUserGamificationHandler,
			coordinator,
			prCoordinatorAdapter{coordinator: coordinator},
			nutritionCoordinatorAdapter{coordinator: coordinator},
			syncUserSnapshotHandler,
		)
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

	if consumerMgr != nil {
		consumerMgr.Start(workerCtx)
	}

	leaderboard.start(workerCtx, &wg)

	shutdown = func() {
		slog.Info("Shutting down background workers...", slog.String("module", "gamification"))
		cancelWorkers()
		if consumerMgr != nil {
			consumerMgr.Wait()
		}
		wg.Wait()
		slog.Info("Background workers shutdown complete.", slog.String("module", "gamification"))
	}

	return grpcHandler, shutdown, nil
}

// connectLeaderboardRedis returns the shared Redis client, or nil (PostgreSQL-only leaderboard)
// when Redis is unavailable.
func connectLeaderboardRedis() *goredis.Client {
	client, err := sharedRedis.GetRegistry().GetClient()
	if err != nil {
		slog.Warn("Leaderboard read model: postgres only (redis unavailable)",
			slog.String("module", "gamification"), slog.String("error", err.Error()))
		return nil
	}
	return client
}

// leaderboardEventsReader returns the reader feeding XpEarned into the leaderboard, or nil when
// Kafka is unavailable.
func leaderboardEventsReader(registry *sharedKafka.Registry) *kafka.Reader {
	if registry == nil {
		slog.Warn("Leaderboard read model: postgres only (kafka not configured)",
			slog.String("module", "gamification"))
		return nil
	}
	reader, err := registry.GetReader(leaderboardConsumerGroup, gamificationEventsTopic)
	if err != nil {
		slog.Warn("Leaderboard read model: postgres only (kafka reader unavailable)",
			slog.String("module", "gamification"), slog.String("error", err.Error()))
		return nil
	}
	return reader
}

type nutritionCoordinatorAdapter struct {
	coordinator *appService.ActivityIngestionCoordinator
}

func (a nutritionCoordinatorAdapter) Handle(ctx context.Context, cmd command.ProcessNutritionActivityCommand) error {
	return a.coordinator.IngestNutritionActivity(ctx, cmd)
}

type prCoordinatorAdapter struct {
	coordinator *appService.ActivityIngestionCoordinator
}

//nolint:gocritic // hugeParam: command struct is an immutable value object
func (a prCoordinatorAdapter) Handle(ctx context.Context, cmd command.ProcessPersonalRecordCommand) error {
	return a.coordinator.IngestPersonalRecord(ctx, cmd)
}

// RegisterConnectHandler registers the Gamification service on the connect.Server.
func RegisterConnectHandler(server *connect.Server, handler *gamificationGRPC.Handler) string {
	gamificationv1serviceconnect.RegisterGamificationServiceHandler(server, handler)
	return "/" + gamificationv1serviceconnect.GamificationServiceName + "/"
}
