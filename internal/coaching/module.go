package coaching

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	coachingEvent "github.com/viethung213/gym-companion/internal/coaching/infrastructure/event"
	coachingKafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/coaching/transport/consumer"
	coachingGrpc "github.com/viethung213/gym-companion/internal/coaching/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service/coachingv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"github.com/viethung213/gym-companion/internal/shared/kafka"
	"gorm.io/gorm"
)

// realClock returns time.Now().UTC() for production use.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// ModuleDeps contains dependencies for coaching module initialization.
type ModuleDeps struct {
	DB             *sql.DB
	GormDB         *gorm.DB
	KafkaRegistry  *kafka.Registry
	GatewayURL     string
	InternalSecret string
	ProfileReader  port.UserProfileReader
	SessionReader  port.WorkoutSessionReader
	CatalogReader  port.ExerciseCatalogReader
	RoadmapRepo    port.RoadmapRepository
	OutboxWriter   port.OutboxWriter
	IDGenerator    port.IDGenerator
}

// Initialize sets up the coaching module (coaching context agent, gRPC services, etc).
func Initialize(ctx context.Context, deps *ModuleDeps) (*coachingGrpc.Server, port.CoachAgent, func(), error) {
	slog.Info("Initializing Coaching Module...")

	if deps.IDGenerator == nil {
		deps.IDGenerator = persistence.UUIDGenerator{}
	}

	var gormDB *gorm.DB = deps.GormDB
	if gormDB == nil && deps.DB != nil {
		var err error
		gormDB, err = database.OpenGorm(deps.DB, nil)
		if err != nil {
			slog.Warn("Failed to wrap *sql.DB into GORM in Coaching Module", slog.String("error", err.Error()))
		}
	}

	if deps.ProfileReader == nil {
		deps.ProfileReader = adapters.NewConnectUserProfileReader(deps.GatewayURL, deps.InternalSecret, nil)
	}

	if deps.SessionReader == nil {
		deps.SessionReader = adapters.NewConnectWorkoutSessionReader(deps.GatewayURL, deps.InternalSecret, nil)
	}

	if deps.CatalogReader == nil {
		deps.CatalogReader = adapters.NewConnectExerciseCatalogReader(deps.GatewayURL, deps.InternalSecret, nil)
	}

	if deps.RoadmapRepo == nil && gormDB != nil {
		deps.RoadmapRepo = persistence.NewRoadmapRepository(gormDB)
	}

	var outboxRepo port.OutboxRepository
	if gormDB != nil {
		outboxRepo = persistence.NewOutboxRepository(gormDB)
	}

	if deps.OutboxWriter == nil && outboxRepo != nil {
		deps.OutboxWriter = coachingEvent.NewOutboxWriter(outboxRepo)
	}

	// Initialize Coaching Context Agent (ADK)
	coachAgent, err := adk.NewCoachingContextAgent(
		ctx,
		deps.ProfileReader,
		deps.SessionReader,
		deps.CatalogReader,
		deps.RoadmapRepo,
		deps.IDGenerator,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize coaching context agent: %w", err)
	}

	var txMgr port.TransactionManager
	if gormDB != nil {
		txMgr = persistence.NewSQLTransactionManager(gormDB)
	}

	// One engine for all users: each Check receives the user's injuries.
	guard := guardrail.NewEngine(service.NewOverloadValidator(), deps.CatalogReader, nil)
	clock := realClock{}

	// Wire gRPC Command & Query Handlers
	var initiateHandler *command.InitiateRoadmapHandler
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil && txMgr != nil {
		initiateHandler = command.NewInitiateRoadmapHandler(
			txMgr,
			deps.RoadmapRepo,
			coachAgent,
			guard,
			deps.ProfileReader,
			deps.OutboxWriter,
			clock,
		)
	}

	var regenerateHandler *command.RegenerateScheduleHandler
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil && txMgr != nil {
		regenerateHandler = command.NewRegenerateScheduleHandler(
			txMgr,
			deps.RoadmapRepo,
			coachAgent,
			guard,
			deps.ProfileReader,
			deps.OutboxWriter,
			clock,
			deps.IDGenerator,
		)
	}

	var createAdhocHandler *command.CreateAdhocSessionHandler
	if deps.RoadmapRepo != nil && deps.CatalogReader != nil && txMgr != nil {
		createAdhocHandler = command.NewCreateAdhocSessionHandler(
			txMgr,
			deps.RoadmapRepo,
			deps.CatalogReader,
			deps.IDGenerator,
			clock,
		)
	}

	var queriesHandler *query.Handlers
	if deps.RoadmapRepo != nil {
		queriesHandler = query.NewHandlers(deps.RoadmapRepo)
	}

	var suggestAdHocHandler *query.SuggestAdHocSessionHandler
	if deps.RoadmapRepo != nil {
		suggestAdHocHandler = query.NewSuggestAdHocSessionHandler(deps.RoadmapRepo, coachAgent, clock)
	}

	var completeSessionHandler *command.CompleteSessionHandler
	var abortSessionHandler *command.AbortSessionHandler
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil && txMgr != nil {
		completeSessionHandler = command.NewCompleteSessionHandler(
			txMgr,
			deps.RoadmapRepo,
			service.NewSCRCalculator(),
			deps.OutboxWriter,
			clock,
		)
		abortSessionHandler = command.NewAbortSessionHandler(
			txMgr,
			deps.RoadmapRepo,
			deps.OutboxWriter,
			clock,
		)
	}

	coachingServer := coachingGrpc.NewServer(
		initiateHandler,
		regenerateHandler,
		createAdhocHandler,
		queriesHandler,
	).WithSuggestAdHoc(suggestAdHocHandler)

	var reminderWorker *worker.UpcomingWorkoutReminderWorker
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil {
		reminderWorker = worker.NewUpcomingWorkoutReminderWorker(deps.RoadmapRepo, deps.OutboxWriter, 5*time.Minute)
		go func() {
			_ = reminderWorker.Start(ctx)
		}()
		slog.Info("Coaching UpcomingWorkoutReminderWorker started")
	} else {
		slog.Warn("Coaching UpcomingWorkoutReminderWorker not started (missing RoadmapRepo or OutboxWriter)")
	}

	var outboxWorker *worker.OutboxWorker
	var consumerMgr *consumer.ConsumerManager

	if deps.KafkaRegistry != nil {
		if outboxRepo != nil {
			writer, wErr := deps.KafkaRegistry.GetWriter("coaching.events")
			if wErr == nil && writer != nil {
				kafkaPub := coachingKafka.NewPublisher(writer)
				outboxWorker = worker.NewOutboxWorker(outboxRepo, kafkaPub, 2*time.Second)
				go outboxWorker.Start(ctx)
				slog.Info("Coaching OutboxWorker started")
			}
		}

		consumerMgr = consumer.NewConsumerManager(
			deps.KafkaRegistry,
			initiateHandler,
			regenerateHandler,
			completeSessionHandler,
			abortSessionHandler,
			outboxRepo,
		)
		if consumerMgr != nil {
			consumerMgr.Start(ctx)
		}
	}

	slog.Info("Coaching Module initialized successfully")

	shutdown := func() {
		slog.Info("Shutting down Coaching Module...")
		if consumerMgr != nil {
			consumerMgr.Wait()
		}
		if reminderWorker != nil {
			reminderWorker.Stop()
		}
	}

	return coachingServer, coachAgent, shutdown, nil
}

// RegisterConnectHandler registers the ConnectRPC handler for Coaching module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	coachingServer *coachingGrpc.Server,
) string {
	if coachingServer == nil {
		return ""
	}
	connectHandler := coachingGrpc.NewConnectCoachingHandler(coachingServer)
	coachingv1serviceconnect.RegisterCoachingServiceHandler(server, connectHandler)
	return "/" + coachingv1serviceconnect.CoachingServiceName + "/"
}
