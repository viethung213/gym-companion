package coaching

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/coaching/application/athlete"
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

// lifecycleInterval is how often roadmaps are checked for ended blocks,
// finished weeks and blocks to plan ahead.
const lifecycleInterval = 30 * time.Minute

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
	// Lifecycle stores the roadmap lifecycle (week reviews, block rotation).
	// It defaults to the roadmap repository built from the database; without
	// it the lifecycle worker does not run.
	Lifecycle    port.RoadmapLifecycleStore
	OutboxWriter port.OutboxWriter
	IDGenerator  port.IDGenerator
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
	// Every flow sees the same assumptions for fields a profile leaves empty.
	deps.ProfileReader = athlete.WithProfileDefaults(deps.ProfileReader)

	if deps.SessionReader == nil {
		deps.SessionReader = adapters.NewConnectWorkoutSessionReader(deps.GatewayURL, deps.InternalSecret, nil)
	}

	if deps.CatalogReader == nil {
		deps.CatalogReader = adapters.NewConnectExerciseCatalogReader(deps.GatewayURL, deps.InternalSecret, nil)
	}

	if deps.RoadmapRepo == nil && gormDB != nil {
		repo := persistence.NewRoadmapRepository(gormDB)
		deps.RoadmapRepo = repo
		if deps.Lifecycle == nil {
			deps.Lifecycle = repo
		}
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
	guard := guardrail.NewEngine(service.NewOverloadValidator(), deps.CatalogReader, athlete.PRBaselines(deps.SessionReader))
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
			deps.ProfileReader,
			deps.IDGenerator,
			clock,
		)
	}

	// Wiring rule: a handler's required collaborators are constructor
	// parameters; an optional one that switches a behaviour on is a WithX setter.
	//
	// Roadmap rotation (command/plan_next_block.go, command/roll_over.go): the
	// next 4-week block is planned during the final week and swapped in when
	// the active one ends.
	lifecycle := deps.Lifecycle
	var rollOverHandler *command.RollOverHandler
	if lifecycle != nil && deps.OutboxWriter != nil && txMgr != nil {
		rollOverHandler = command.NewRollOverHandler(txMgr, deps.RoadmapRepo, lifecycle, deps.OutboxWriter, clock)
		if regenerateHandler != nil {
			regenerateHandler.WithPlannedBlocks(lifecycle)
		}
		if createAdhocHandler != nil {
			createAdhocHandler.WithRollover(rollOverHandler)
			if initiateHandler != nil {
				createAdhocHandler.WithFreshStart(initiateHandler)
			}
		}
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

	// Keep roadmaps moving: roll over ended blocks, review finished weeks and
	// adapt to them (BR-AC-04), plan the next block during the final week.
	var lifecycleWorker *worker.RoadmapLifecycleWorker
	if rollOverHandler != nil {
		adaptHandler := command.NewApplyAdaptiveCycleHandler(
			txMgr, deps.RoadmapRepo, coachAgent, guard, deps.ProfileReader, deps.OutboxWriter, clock,
			nil, // engine: the default AdaptiveCoachEngine
		)
		reviewHandler := command.NewReviewWeeksHandler(txMgr, deps.RoadmapRepo, adaptHandler, clock)
		planHandler := command.NewPlanNextBlockHandler(
			txMgr, deps.RoadmapRepo, lifecycle, coachAgent, guard, deps.ProfileReader, clock,
		)
		lifecycleWorker = worker.NewRoadmapLifecycleWorker(lifecycle, rollOverHandler, reviewHandler, planHandler, lifecycleInterval)
		if initiateHandler != nil {
			lifecycleWorker.WithInitiator(initiateHandler)
		}
		if regenerateHandler != nil {
			lifecycleWorker.WithRegenerator(regenerateHandler)
		}
		lifecycleWorker.Start(ctx)
	} else {
		slog.Warn("Coaching RoadmapLifecycleWorker not started (no Lifecycle store, OutboxWriter or database)")
	}

	var outboxWorker *worker.OutboxWorker
	var profileConsumer *consumer.ProfileCompletedConsumer
	var profileConsumerCancel context.CancelFunc
	var workoutConsumer *consumer.WorkoutExecutionConsumer
	var workoutConsumerCancel context.CancelFunc

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

		// Subscribe to profile.events for auto-initiation / regeneration of roadmaps.
		if initiateHandler != nil && outboxRepo != nil {
			reader, rErr := deps.KafkaRegistry.GetReader(
				"coaching-profile-completed-group", "profile.events",
			)
			if rErr == nil && reader != nil {
				profileConsumer = consumer.NewProfileCompletedConsumer(reader, initiateHandler, regenerateHandler, outboxRepo)
				var consumerCtx context.Context
				consumerCtx, profileConsumerCancel = context.WithCancel(ctx)
				go profileConsumer.Start(consumerCtx)
				slog.Info("Coaching ProfileCompletedConsumer started (profile.events)")
			} else {
				slog.Warn("Coaching ProfileCompletedConsumer not started", slog.Any("reader_error", rErr))
			}
		} else {
			slog.Warn("Coaching ProfileCompletedConsumer not started (missing InitiateRoadmapHandler or OutboxRepo)")
		}

		// Subscribe to workout_execution.events for session completion / abortion.
		if completeSessionHandler != nil && abortSessionHandler != nil && outboxRepo != nil {
			weReader, weErr := deps.KafkaRegistry.GetReader(
				"coaching-workout-execution-group", "workout_execution.events",
			)
			if weErr == nil && weReader != nil {
				workoutConsumer = consumer.NewWorkoutExecutionConsumer(
					weReader,
					completeSessionHandler,
					abortSessionHandler,
					outboxRepo,
				)
				var weCtx context.Context
				weCtx, workoutConsumerCancel = context.WithCancel(ctx)
				go workoutConsumer.Start(weCtx)
				slog.Info("Coaching WorkoutExecutionConsumer started (workout_execution.events)")
			} else {
				slog.Warn("Coaching WorkoutExecutionConsumer not started", slog.Any("reader_error", weErr))
			}
		} else {
			slog.Warn("Coaching WorkoutExecutionConsumer not started (missing CompleteSessionHandler, AbortSessionHandler or OutboxRepo)")
		}
	}

	slog.Info("Coaching Module initialized successfully")

	shutdown := func() {
		slog.Info("Shutting down Coaching Module...")
		if profileConsumerCancel != nil {
			profileConsumerCancel()
		}
		if workoutConsumerCancel != nil {
			workoutConsumerCancel()
		}
		if reminderWorker != nil {
			reminderWorker.Stop()
		}
		if lifecycleWorker != nil {
			lifecycleWorker.Stop()
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
