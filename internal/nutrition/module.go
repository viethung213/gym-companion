package nutrition

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service/nutritionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	nutritionAdk "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/ai/adk"
	nutritionEvent "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/event"
	nutritionKafka "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/profileclient"
	nutritionRedis "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/redis"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/worker"
	nutritionConsumer "github.com/viethung213/gym-companion/internal/nutrition/transport/consumer"
	nutritionGRPC "github.com/viethung213/gym-companion/internal/nutrition/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB             *sql.DB
	KafkaRegistry  *sharedKafka.Registry
	RedisClient    *redis.Client
	GatewayURL     string
	InternalSecret string
}

type Module struct {
	GRPCHandler               *nutritionGRPC.GRPCHandler
	NutritionInsightHdlr      *query.GetNutritionInsightHandler
	GenerateWeeklyInsightHdlr *command.GenerateWeeklyInsightHandler
	OutboxWorker              *worker.OutboxWorker
	CronWorker                *worker.DailyMenuCronWorker
	WeeklyInsightWorker       *worker.WeeklyInsightCronWorker
	MealReminderWorker        *worker.UpcomingMealReminderWorker
	ConsumerManager           *nutritionConsumer.ConsumerManager
	KafkaPub                  *nutritionKafka.Publisher
	TxManager                 *persistence.SQLTransactionManager
}

func NewModule(
	ctx context.Context,
	db *gorm.DB,
	kafkaRegistry *sharedKafka.Registry,
	profileCli repository.ProfileClient,
	cronCoordinator repository.CronTaskCoordinator,
) *Module {
	txManager := persistence.NewSQLTransactionManager(db)

	foodRepo := persistence.NewPostgresFoodItemRepository(db)
	recipeCacheRepo := persistence.NewPostgresRecipeCacheRepository(db)
	planRepo := persistence.NewPostgresNutritionPlanRepository(db)
	historyRepo := persistence.NewPostgresMealHistoryRepository(db)
	weeklyInsightRepo := persistence.NewPostgresWeeklyInsightRepository(db)
	outboxRepo := persistence.NewPostgresOutboxRepository(db)
	outboxLogRepo := persistence.NewPostgresOutboxLogRepository(db)

	outboxWriter := nutritionEvent.NewOutboxWriter(outboxRepo)
	aiAgent, err := nutritionAdk.NewNutritionAgent(ctx, foodRepo)
	if err != nil {
		slog.Warn("ADK agent init failed (fallback)", slog.String("module", "nutrition"), slog.String("error", err.Error()))
		aiAgent = nil
	}

	tdeeCalc := service.NewTDEECalculator()
	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, aiAgent)

	genPlanHdlr := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, outboxWriter, tdeeCalc, menuGen, txManager).
		WithWeeklyInsightRepo(weeklyInsightRepo)
	recalPlanHdlr := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, outboxWriter, menuGen, txManager)
	rebalanceWorkoutHdlr := command.NewRebalanceWorkoutNutritionHandler(planRepo, outboxWriter, txManager)
	logMealHdlr := command.NewLogMealHandler(planRepo, historyRepo, outboxWriter, aiAgent, txManager)
	createFoodItemHdlr := command.NewCreateFoodItemHandler(foodRepo)
	approveFoodItemHdlr := command.NewApproveFoodItemHandler(foodRepo)
	updateMealScheduleHdlr := command.NewUpdateMealScheduleHandler(planRepo, txManager)
	generateWeeklyInsightHdlr := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepo, weeklyInsightRepo, aiAgent)

	getTodayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	getNutritionHistHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	getNutritionSummHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)
	getNutritionInsightHdlr := query.NewGetNutritionInsightHandler(weeklyInsightRepo)

	allergyRepo := persistence.NewPostgresUserAllergyRepository(db)
	getUserAllergiesHdlr := query.NewGetUserAllergiesHandler(allergyRepo)
	updateUserAllergiesHdlr := command.NewUpdateUserAllergiesHandler(allergyRepo)

	grpcHdlr := nutritionGRPC.NewGRPCHandler(
		genPlanHdlr,
		recalPlanHdlr,
		logMealHdlr,
		createFoodItemHdlr,
		approveFoodItemHdlr,
		updateMealScheduleHdlr,
		getTodayMenuHdlr,
		getNutritionHistHdlr,
		getNutritionSummHdlr,
		getNutritionInsightHdlr,
	).WithAllergyHandlers(
		getUserAllergiesHdlr,
		updateUserAllergiesHdlr,
	)

	var kafkaPub *nutritionKafka.Publisher
	var consumerMgr *nutritionConsumer.ConsumerManager

	if kafkaRegistry != nil {
		writer, wErr := kafkaRegistry.GetWriter("nutrition.events")
		if wErr == nil && writer != nil {
			kafkaPub = nutritionKafka.NewPublisher(writer)
		}

		consumerMgr = nutritionConsumer.NewConsumerManager(
			kafkaRegistry,
			planRepo,
			outboxLogRepo,
			genPlanHdlr,
			allergyRepo,
			rebalanceWorkoutHdlr,
		)
	}

	outboxWorker := worker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
	cronWorker := worker.NewDailyMenuCronWorker(genPlanHdlr, planRepo, profileCli, cronCoordinator, 5, 0).
		WithUserAllergyRepository(allergyRepo)
	weeklyInsightWorker := worker.NewWeeklyInsightCronWorker(generateWeeklyInsightHdlr, planRepo, cronCoordinator, 23, 0)
	mealReminderWorker := worker.NewUpcomingMealReminderWorker(planRepo, outboxWriter, 5*time.Minute)

	return &Module{
		GRPCHandler:               grpcHdlr,
		NutritionInsightHdlr:      getNutritionInsightHdlr,
		GenerateWeeklyInsightHdlr: generateWeeklyInsightHdlr,
		OutboxWorker:              outboxWorker,
		CronWorker:                cronWorker,
		WeeklyInsightWorker:       weeklyInsightWorker,
		MealReminderWorker:        mealReminderWorker,
		ConsumerManager:           consumerMgr,
		KafkaPub:                  kafkaPub,
		TxManager:                 txManager,
	}
}

func Initialize(ctx context.Context, deps ModuleDeps) (*nutritionGRPC.GRPCHandler, func(), error) {
	var gormDB *gorm.DB
	if deps.DB != nil {
		gdb, err := database.OpenGorm(deps.DB, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("nutrition module gorm open: %w", err)
		}
		gormDB = gdb
	}

	var profileCli repository.ProfileClient
	if deps.GatewayURL != "" {
		profileCli = profileclient.NewConnectProfileClient(deps.GatewayURL, deps.InternalSecret, nil)
	}

	var cronCoordinator repository.CronTaskCoordinator
	if deps.RedisClient != nil {
		cronCoordinator = nutritionRedis.NewCronTaskCoordinator(deps.RedisClient)
	}

	mod := NewModule(ctx, gormDB, deps.KafkaRegistry, profileCli, cronCoordinator)

	workerCtx, cancel := context.WithCancel(ctx)
	mod.StartWorkers(workerCtx)

	shutdown := func() {
		cancel()
		mod.CronWorker.Stop()
		if mod.WeeklyInsightWorker != nil {
			mod.WeeklyInsightWorker.Stop()
		}
		if mod.MealReminderWorker != nil {
			mod.MealReminderWorker.Stop()
		}
		if mod.ConsumerManager != nil {
			mod.ConsumerManager.Wait()
		}
		if mod.KafkaPub != nil {
			_ = mod.KafkaPub.Close()
		}
		slog.Info("Shutdown completed cleanly", slog.String("module", "nutrition"))
	}

	return mod.GRPCHandler, shutdown, nil
}

// RegisterConnectHandler registers the ConnectRPC handler for the Nutrition module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	grpcHandler *nutritionGRPC.GRPCHandler,
) string {
	connectHandler := nutritionGRPC.NewConnectNutritionHandler(grpcHandler)
	nutritionv1serviceconnect.RegisterNutritionServiceHandler(server, connectHandler)
	return "/" + nutritionv1serviceconnect.NutritionServiceName + "/"
}

func (m *Module) StartWorkers(ctx context.Context) {
	go func() {
		_ = m.OutboxWorker.Start(ctx)
	}()
	go func() {
		_ = m.CronWorker.Start(ctx)
	}()
	if m.WeeklyInsightWorker != nil {
		go func() {
			_ = m.WeeklyInsightWorker.Start(ctx)
		}()
	}
	if m.MealReminderWorker != nil {
		go func() {
			_ = m.MealReminderWorker.Start(ctx)
		}()
	}
	if m.ConsumerManager != nil {
		m.ConsumerManager.Start(ctx)
	}
}
