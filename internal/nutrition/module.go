package nutrition

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service/nutritionv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	nutritionAdk "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/ai/adk"
	nutritionConfig "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	nutritionEvent "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/event"
	nutritionKafka "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/profileclient"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/worker"
	nutritionConsumer "github.com/viethung213/gym-companion/internal/nutrition/transport/consumer"
	nutritionGRPC "github.com/viethung213/gym-companion/internal/nutrition/transport/grpc"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
	// ProfileConn là kết nối gRPC in-process đến ProfileService để lấy dữ liệu sinh trắc học.
	// Có thể nil — khi đó DailyMenuCronWorker dùng fallback metrics.
	ProfileConn grpc.ClientConnInterface
}

type Module struct {
	GRPCHandler          *nutritionGRPC.GRPCHandler
	OutboxWorker         *worker.OutboxWorker
	CronWorker           *worker.DailyMenuCronWorker
	MealReminderWorker   *worker.UpcomingMealReminderWorker
	ProfileEventConsumer *nutritionConsumer.ProfileEventConsumer
	KafkaConsumer        *nutritionKafka.Consumer
	KafkaPub             *nutritionKafka.Publisher
	TxManager            *persistence.SQLTransactionManager
}

func NewModule(ctx context.Context, db *gorm.DB, aiAPIKey string, kafkaRegistry *sharedKafka.Registry, profileCli repository.ProfileClient) *Module {
	txManager := persistence.NewSQLTransactionManager(db)

	foodRepo := persistence.NewPostgresFoodItemRepository(db)
	recipeCacheRepo := persistence.NewPostgresRecipeCacheRepository(db)
	planRepo := persistence.NewPostgresNutritionPlanRepository(db)
	historyRepo := persistence.NewPostgresMealHistoryRepository(db)
	outboxRepo := persistence.NewPostgresOutboxRepository(db)
	outboxLogRepo := persistence.NewPostgresOutboxLogRepository(db)

	outboxWriter := nutritionEvent.NewOutboxWriter(outboxRepo)
	aiAgent, err := nutritionAdk.NewNutritionAgent(ctx, aiAPIKey, foodRepo)
	if err != nil {
		log.Printf("[Nutrition Module] ADK agent init failed (fallback): %v", err)
		aiAgent = nil
	}

	tdeeCalc := service.NewTDEECalculator()
	matrix := service.NewCombinatorialMatrix()
	menuGen := service.NewMenuGenerator(matrix, recipeCacheRepo, foodRepo, aiAgent)

	genPlanHdlr := command.NewGenerateDailyPlanHandler(planRepo, historyRepo, outboxWriter, tdeeCalc, menuGen)
	recalPlanHdlr := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, outboxWriter, menuGen)
	logMealHdlr := command.NewLogMealHandler(planRepo, historyRepo, outboxWriter, aiAgent)
	createFoodItemHdlr := command.NewCreateFoodItemHandler(foodRepo)
	approveFoodItemHdlr := command.NewApproveFoodItemHandler(foodRepo)
	updateMealScheduleHdlr := command.NewUpdateMealScheduleHandler(planRepo)

	getTodayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	getNutritionHistHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	getNutritionSummHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)
	getNutritionInsightHdlr := query.NewGetNutritionInsightHandler(planRepo, historyRepo, aiAgent)

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
	)

	var kafkaPub *nutritionKafka.Publisher
	var kafkaConsumer *nutritionKafka.Consumer
	var profileEventConsumer *nutritionConsumer.ProfileEventConsumer

	if kafkaRegistry != nil {
		cfg := nutritionConfig.LoadConfig()
		brokers := strings.Split(cfg.KafkaBrokers, ",")

		writer, wErr := kafkaRegistry.GetWriter("nutrition.events", brokers)
		if wErr == nil && writer != nil {
			kafkaPub = nutritionKafka.NewPublisher(writer)
		}

		reader, rErr := kafkaRegistry.GetReader("nutrition-workout-completed-group", "workout_execution.events", brokers)
		if rErr == nil && reader != nil {
			kafkaConsumer = nutritionKafka.NewConsumer(reader, recalPlanHdlr)
		}

		profileEventReader, pErr := kafkaRegistry.GetReader("nutrition-profile-updated-group", "profile.events", brokers)
		if pErr == nil && profileEventReader != nil {
			profileEventConsumer = nutritionConsumer.NewProfileEventConsumer(profileEventReader, planRepo, genPlanHdlr)
		}
	}

	outboxWorker := worker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
	cronWorker := worker.NewDailyMenuCronWorker(genPlanHdlr, planRepo, profileCli)
	mealReminderWorker := worker.NewUpcomingMealReminderWorker(planRepo, outboxWriter, 5*time.Minute)

	return &Module{
		GRPCHandler:          grpcHdlr,
		OutboxWorker:         outboxWorker,
		CronWorker:           cronWorker,
		MealReminderWorker:   mealReminderWorker,
		ProfileEventConsumer: profileEventConsumer,
		KafkaConsumer:        kafkaConsumer,
		KafkaPub:             kafkaPub,
		TxManager:            txManager,
	}
}

func Initialize(ctx context.Context, deps ModuleDeps) (*nutritionGRPC.GRPCHandler, func(), error) {
	var gormDB *gorm.DB
	if deps.DB != nil {
		gdb, err := gorm.Open(gormPostgres.New(gormPostgres.Config{Conn: deps.DB}), &gorm.Config{})
		if err != nil {
			return nil, nil, fmt.Errorf("nutrition module gorm open: %w", err)
		}
		gormDB = gdb
	}

	var profileCli *profileclient.GRPCProfileClient
	if deps.ProfileConn != nil {
		profileCli = profileclient.NewGRPCProfileClient(deps.ProfileConn)
	}

	mod := NewModule(ctx, gormDB, "", deps.KafkaRegistry, profileCli)

	workerCtx, cancel := context.WithCancel(ctx)
	mod.StartWorkers(workerCtx)

	shutdown := func() {
		cancel()
		mod.CronWorker.Stop()
		if mod.MealReminderWorker != nil {
			mod.MealReminderWorker.Stop()
		}
		if mod.KafkaPub != nil {
			_ = mod.KafkaPub.Close()
		}
		log.Println("[Nutrition Module] Shutdown completed cleanly")
	}

	return mod.GRPCHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for the Nutrition module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *nutritionGRPC.GRPCHandler,
	opts ...connect.HandlerOption,
) {
	connectHandler := nutritionGRPC.NewConnectNutritionHandler(grpcHandler)
	path, handler := nutritionv1serviceconnect.NewNutritionServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}

func (m *Module) StartWorkers(ctx context.Context) {
	go func() {
		_ = m.OutboxWorker.Start(ctx)
	}()
	go func() {
		_ = m.CronWorker.Start(ctx)
	}()
	if m.MealReminderWorker != nil {
		go func() {
			_ = m.MealReminderWorker.Start(ctx)
		}()
	}
	if m.ProfileEventConsumer != nil {
		go func() {
			_ = m.ProfileEventConsumer.Start(ctx)
		}()
	}
	if m.KafkaConsumer != nil {
		go func() {
			_ = m.KafkaConsumer.Start(ctx)
		}()
	}
}
