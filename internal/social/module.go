package social

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
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/service/socialv1serviceconnect"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/query"
	socialEvent "github.com/viethung213/gym-companion/internal/social/infrastructure/event"
	socialKafka "github.com/viethung213/gym-companion/internal/social/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/worker"
	socialConsumer "github.com/viethung213/gym-companion/internal/social/transport/consumer"
	socialGRPC "github.com/viethung213/gym-companion/internal/social/transport/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*socialGRPC.GRPCHandler, func(), error) {
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wrap social db pool in gorm: %w", err)
	}

	followRepo := persistence.NewGormFollowRepository(gormDB)
	feedItemRepo := persistence.NewGormFeedItemRepository(gormDB)
	interactionRepo := persistence.NewGormInteractionRepository(gormDB)
	outboxRepo := persistence.NewGormOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewGormOutboxLogRepository(gormDB)
	txManager := persistence.NewSQLTransactionManager(gormDB)
	eventPub := socialEvent.NewOutboxWriter(outboxRepo)
	userSnapshotRepo := persistence.NewGormUserSnapshotRepository(gormDB)

	// Command Handlers
	followUserHandler := command.NewFollowUserHandler(followRepo, userSnapshotRepo, eventPub, txManager)
	unfollowUserHandler := command.NewUnfollowUserHandler(followRepo, txManager)
	createPostHandler := command.NewCreatePostHandler(
		feedItemRepo,
		eventPub,
		txManager,
		command.WithCreatePostFollowRepo(followRepo),
		command.WithCreatePostUserSnapshotRepo(userSnapshotRepo),
	)
	deleteFeedItemHandler := command.NewDeleteFeedItemHandler(feedItemRepo, txManager)
	reactTargetHandler := command.NewReactTargetHandler(interactionRepo, feedItemRepo, userSnapshotRepo, eventPub, txManager)
	addCommentHandler := command.NewAddCommentHandler(interactionRepo, feedItemRepo, userSnapshotRepo, eventPub, txManager)
	deleteCommentHandler := command.NewDeleteCommentHandler(interactionRepo, feedItemRepo, txManager)
	ingestWorkoutActivityHandler := command.NewIngestWorkoutActivityHandler(feedItemRepo, txManager)
	syncUserSnapshotHandler := command.NewSyncUserSnapshotHandler(userSnapshotRepo, txManager)
	updateUserRoleHandler := command.NewUpdateUserRoleHandler(userSnapshotRepo, txManager)

	// Query Handlers - using local social.users snapshot
	getActivityFeedHandler := query.NewGetActivityFeedHandler(followRepo, feedItemRepo, interactionRepo, userSnapshotRepo)
	getUserProfileFeedHandler := query.NewGetUserProfileFeedHandler(feedItemRepo, userSnapshotRepo)
	getSocialSummaryHandler := query.NewGetSocialSummaryHandler(followRepo, feedItemRepo)
	getFollowersHandler := query.NewGetFollowersHandler(followRepo, userSnapshotRepo)
	getFollowingHandler := query.NewGetFollowingHandler(followRepo, userSnapshotRepo)
	listCommentsHandler := query.NewListCommentsHandler(interactionRepo, userSnapshotRepo)
	listReactionsHandler := query.NewListReactionsHandler(interactionRepo, userSnapshotRepo)
	searchUsersHandler := query.NewSearchUsersHandler(userSnapshotRepo, followRepo)

	// Transport Handler
	grpcHandler := socialGRPC.NewGRPCHandler(
		followUserHandler,
		unfollowUserHandler,
		createPostHandler,
		deleteFeedItemHandler,
		reactTargetHandler,
		addCommentHandler,
		deleteCommentHandler,
		getActivityFeedHandler,
		getUserProfileFeedHandler,
		getSocialSummaryHandler,
		getFollowersHandler,
		getFollowingHandler,
		listCommentsHandler,
		listReactionsHandler,
		searchUsersHandler,
	)

	// Background workers (Kafka publisher & consumers)
	var kafkaPub *socialKafka.Publisher
	var workoutSharedConsumer *socialConsumer.WorkoutSharedConsumer
	var userRegisteredConsumer *socialConsumer.UserRegisteredConsumer
	var userIdentityUpdatedConsumer *socialConsumer.UserIdentityUpdatedConsumer
	var userRoleUpdatedConsumer *socialConsumer.UserRoleUpdatedConsumer

	if deps.KafkaRegistry != nil {
		kafkaBrokers := os.Getenv("KAFKA_BROKERS")
		if kafkaBrokers == "" {
			kafkaBrokers = "localhost:9092"
		}
		brokers := strings.Split(kafkaBrokers, ",")

		notifWriter, nErr := deps.KafkaRegistry.GetWriter("notification.events.normal-priority", brokers)
		if nErr != nil {
			log.Printf("Warning: failed to get kafka writer for notification topic: %v", nErr)
		} else if notifWriter != nil {
			kafkaPub = socialKafka.NewPublisher(notifWriter)
		}

		workoutReader, rErr := deps.KafkaRegistry.GetReader("social-workout-shared-group", "workout_execution.events", brokers)
		if rErr == nil && workoutReader != nil {
			workoutSharedConsumer = socialConsumer.NewWorkoutSharedConsumer(workoutReader, ingestWorkoutActivityHandler, outboxLogRepo, txManager)
		}

		authReader, aErr := deps.KafkaRegistry.GetReader("social-user-registered-group", "auth.events", brokers)
		if aErr == nil && authReader != nil {
			userRegisteredConsumer = socialConsumer.NewUserRegisteredConsumer(authReader, syncUserSnapshotHandler, outboxLogRepo)
		}

		authRoleReader, arErr := deps.KafkaRegistry.GetReader("social-user-role-updated-group", "auth.events", brokers)
		if arErr == nil && authRoleReader != nil {
			userRoleUpdatedConsumer = socialConsumer.NewUserRoleUpdatedConsumer(authRoleReader, updateUserRoleHandler, outboxLogRepo)
		}

		profileReader, pErr := deps.KafkaRegistry.GetReader("social-user-identity-updated-group", "profile.events", brokers)
		if pErr == nil && profileReader != nil {
			userIdentityUpdatedConsumer = socialConsumer.NewUserIdentityUpdatedConsumer(profileReader, syncUserSnapshotHandler, outboxLogRepo)
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

	if workoutSharedConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workoutSharedConsumer.Start(workerCtx)
		}()
	}

	if userRegisteredConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userRegisteredConsumer.Start(workerCtx)
		}()
	}

	if userRoleUpdatedConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userRoleUpdatedConsumer.Start(workerCtx)
		}()
	}

	if userIdentityUpdatedConsumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			userIdentityUpdatedConsumer.Start(workerCtx)
		}()
	}

	shutdown := func() {
		log.Println("Shutting down Social Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		log.Println("Social Bounded Context successfully shutdown.")
	}

	return grpcHandler, shutdown, nil
}

func RegisterConnectHandler(mux *http.ServeMux, handler *socialGRPC.GRPCHandler, opts ...connect.HandlerOption) {
	path, connectHandler := socialv1serviceconnect.NewSocialServiceHandler(handler, opts...)
	mux.Handle(path, connectHandler)
}
