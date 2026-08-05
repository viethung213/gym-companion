package nutrition

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	nutritionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	nutritionAdk "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/ai/adk"
	nutritionEvent "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/event"
	nutritionKafka "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/profileclient"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/worker"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB            *sql.DB
	GRPCServer    *grpc.Server
	KafkaRegistry *sharedKafka.Registry
	// ProfileConn là kết nối gRPC in-process đến ProfileService để lấy dữ liệu sinh trắc học.
	// Có thể nil — khi đó DailyMenuCronWorker dùng fallback metrics.
	ProfileConn   grpc.ClientConnInterface
}

type Module struct {
	GRPCHandler   *transport.GRPCHandler
	OutboxWorker  *worker.OutboxWorker
	CronWorker    *worker.DailyMenuCronWorker
	KafkaConsumer *nutritionKafka.Consumer
	KafkaPub      *nutritionKafka.Publisher
	TxManager     *persistence.SQLTransactionManager
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
	aiAgent, err := nutritionAdk.NewADKNutritionAgent(ctx, aiAPIKey, foodRepo)
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

	getTodayMenuHdlr := query.NewGetTodayMenuHandler(planRepo)
	getNutritionHistHdlr := query.NewGetNutritionHistoryHandler(historyRepo)
	getNutritionSummHdlr := query.NewGetNutritionSummaryHandler(planRepo, historyRepo)
	getNutritionInsightHdlr := query.NewGetNutritionInsightHandler(planRepo, historyRepo, aiAgent)

	grpcHdlr := transport.NewGRPCHandler(
		genPlanHdlr,
		recalPlanHdlr,
		logMealHdlr,
		createFoodItemHdlr,
		approveFoodItemHdlr,
		getTodayMenuHdlr,
		getNutritionHistHdlr,
		getNutritionSummHdlr,
		getNutritionInsightHdlr,
	)

	var kafkaPub *nutritionKafka.Publisher
	var kafkaConsumer *nutritionKafka.Consumer

	if kafkaRegistry != nil {
		brokersStr := os.Getenv("KAFKA_BROKERS")
		if brokersStr == "" {
			brokersStr = "localhost:9092"
		}
		brokers := strings.Split(brokersStr, ",")

		writer, wErr := kafkaRegistry.GetWriter("nutrition-events", brokers)
		if wErr == nil && writer != nil {
			kafkaPub = nutritionKafka.NewPublisher(writer)
		}

		reader, rErr := kafkaRegistry.GetReader("workout.session.completed", "nutrition-consumer-group", brokers)
		if rErr == nil && reader != nil {
			kafkaConsumer = nutritionKafka.NewConsumer(reader, recalPlanHdlr)
		}
	}

	outboxWorker := worker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
	cronWorker := worker.NewDailyMenuCronWorker(genPlanHdlr, planRepo, profileCli)

	return &Module{
		GRPCHandler:   grpcHdlr,
		OutboxWorker:  outboxWorker,
		CronWorker:    cronWorker,
		KafkaConsumer: kafkaConsumer,
		KafkaPub:      kafkaPub,
		TxManager:     txManager,
	}
}

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	var gormDB *gorm.DB
	if deps.DB != nil {
		gdb, err := gorm.Open(gormPostgres.New(gormPostgres.Config{Conn: deps.DB}), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("nutrition module gorm open: %w", err)
		}
		gormDB = gdb
	}

	var profileCli *profileclient.GRPCProfileClient
	if deps.ProfileConn != nil {
		profileCli = profileclient.NewGRPCProfileClient(deps.ProfileConn)
	}

	mod := NewModule(ctx, gormDB, "", deps.KafkaRegistry, profileCli)
	if deps.GRPCServer != nil {
		nutritionv1service.RegisterNutritionServiceServer(deps.GRPCServer, mod.GRPCHandler)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	mod.StartWorkers(workerCtx)

	shutdown := func() {
		cancel()
		mod.CronWorker.Stop()
		if mod.KafkaPub != nil {
			_ = mod.KafkaPub.Close()
		}
		log.Println("[Nutrition Module] Shutdown completed cleanly")
	}

	return shutdown, nil
}

func RegisterGateway(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
	if err := nutritionv1service.RegisterNutritionServiceHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
		return fmt.Errorf("nutrition module gateway registration: %w", err)
	}
	return nil
}

func (m *Module) StartWorkers(ctx context.Context) {
	go func() {
		_ = m.OutboxWorker.Start(ctx)
	}()
	go func() {
		_ = m.CronWorker.Start(ctx)
	}()
	if m.KafkaConsumer != nil {
		go func() {
			_ = m.KafkaConsumer.Start(ctx)
		}()
	}
}
