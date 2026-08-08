package coaching

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai/adk"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/config"
	coachingEvent "github.com/viethung213/gym-companion/internal/coaching/infrastructure/event"
	coachingKafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/coaching/transport/consumer"
	coachingGrpc "github.com/viethung213/gym-companion/internal/coaching/transport/grpc"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service/coachingv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/kafka"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// realClock returns time.Now() for production use.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// ModuleDeps contains dependencies for coaching module initialization.
type ModuleDeps struct {
	DB            *sql.DB
	GormDB        *gorm.DB
	KafkaRegistry *kafka.Registry
	ProfileReader port.UserProfileReader
	SessionReader port.WorkoutSessionReader
	CatalogReader port.ExerciseCatalogReader
	RoadmapRepo   port.RoadmapRepository
	OutboxWriter  port.OutboxWriter
	IDGenerator   port.IDGenerator
}

// Initialize sets up the coaching module (coaching context agent, gRPC services, etc).
func Initialize(ctx context.Context, deps *ModuleDeps) (*coachingGrpc.Server, port.CoachAgent, func(), error) {
	log.Println("Initializing Coaching Module...")

	if deps.IDGenerator == nil {
		deps.IDGenerator = persistence.UUIDGenerator{}
	}

	var gormDB *gorm.DB = deps.GormDB
	if gormDB == nil && deps.DB != nil {
		var err error
		gormDB, err = gorm.Open(gormPostgres.New(gormPostgres.Config{Conn: deps.DB}), &gorm.Config{})
		if err != nil {
			log.Printf("[Coaching Module] Warning: Failed to wrap *sql.DB into GORM: %v", err)
		}
	}

	if deps.ProfileReader == nil {
		if gormDB != nil {
			deps.ProfileReader = adapters.NewPostgresUserProfileReader(gormDB)
		} else {
			deps.ProfileReader = &adapters.MockUserProfileReader{}
		}
	}

	if deps.SessionReader == nil {
		if gormDB != nil {
			deps.SessionReader = adapters.NewPostgresWorkoutSessionReader(gormDB)
		} else {
			deps.SessionReader = &adapters.MockWorkoutSessionReader{}
		}
	}

	if deps.CatalogReader == nil {
		if gormDB != nil {
			deps.CatalogReader = adapters.NewPostgresExerciseCatalogReader(gormDB)
		} else {
			deps.CatalogReader = &adapters.MockExerciseCatalogReader{}
		}
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

	guard := guardrail.NewEngine(service.NewOverloadValidator(), nil, nil)
	clock := realClock{}

	// Wire gRPC Command & Query Handlers
	var initiateHandler *command.InitiateRoadmapHandler
	if deps.RoadmapRepo != nil && deps.OutboxWriter != nil && txMgr != nil {
		initiateHandler = command.NewInitiateRoadmapHandler(
			txMgr,
			deps.RoadmapRepo,
			coachAgent,
			guard,
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
			deps.OutboxWriter,
			clock,
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
		log.Println("✅ Coaching UpcomingWorkoutReminderWorker started")
	} else {
		log.Println("⚠️ Coaching UpcomingWorkoutReminderWorker not started (missing RoadmapRepo or OutboxWriter)")
	}

	var outboxWorker *worker.OutboxWorker
	var profileConsumer *consumer.ProfileCompletedConsumer
	var profileConsumerCancel context.CancelFunc

	if deps.KafkaRegistry != nil {
		cfg := config.LoadConfig()
		brokers := strings.Split(cfg.KafkaBrokers, ",")

		if outboxRepo != nil {
			writer, wErr := deps.KafkaRegistry.GetWriter("coaching.events", brokers)
			if wErr == nil && writer != nil {
				kafkaPub := coachingKafka.NewPublisher(writer)
				outboxWorker = worker.NewOutboxWorker(outboxRepo, kafkaPub, 2*time.Second)
				go outboxWorker.Start(ctx)
				log.Println("✅ Coaching OutboxWorker started")
			}
		}

		// Subscribe to profile.events for auto-initiation of roadmaps.
		if initiateHandler != nil && outboxRepo != nil {
			reader, rErr := deps.KafkaRegistry.GetReader(
				"coaching-profile-completed-group", "profile.events", brokers,
			)
			if rErr == nil && reader != nil {
				profileConsumer = consumer.NewProfileCompletedConsumer(reader, initiateHandler, outboxRepo)
				var consumerCtx context.Context
				consumerCtx, profileConsumerCancel = context.WithCancel(ctx)
				go profileConsumer.Start(consumerCtx)
				log.Println("✅ Coaching ProfileCompletedConsumer started (profile.events)")
			} else {
				log.Printf("⚠️ Coaching ProfileCompletedConsumer not started: reader error=%v", rErr)
			}
		} else {
			log.Println("⚠️ Coaching ProfileCompletedConsumer not started (missing InitiateRoadmapHandler or OutboxRepo)")
		}
	}

	log.Println("✅ Coaching Module initialized successfully")

	shutdown := func() {
		log.Println("Shutting down Coaching Module...")
		if profileConsumerCancel != nil {
			profileConsumerCancel()
		}
		if reminderWorker != nil {
			reminderWorker.Stop()
		}
	}

	return coachingServer, coachAgent, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for Coaching module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	server *coachingGrpc.Server,
	opts ...connect.HandlerOption,
) {
	if server == nil {
		return
	}
	connectHandler := coachingGrpc.NewConnectCoachingHandler(server)
	path, handler := coachingv1serviceconnect.NewCoachingServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
