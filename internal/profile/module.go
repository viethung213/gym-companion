package profile

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	profilev1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	profileEvent "github.com/viethung213/gym-companion/internal/profile/infrastructure/event"
	profileKafka "github.com/viethung213/gym-companion/internal/profile/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/persistence"
	grpcProfile "github.com/viethung213/gym-companion/internal/profile/infrastructure/transport/grpc"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/worker"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type ModuleDeps struct {
	DB            *sql.DB
	GRPCServer    *grpc.Server
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	userRepo := persistence.NewPostgresUserProfileRepository(gormDB)
	outboxRepo := persistence.NewGormOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewGormOutboxLogRepository(gormDB)
	txManager := persistence.NewSQLTransactionManager(gormDB)
	eventPub := profileEvent.NewOutboxWriter(outboxRepo)

	saveHealthProfileHandler := command.NewSaveHealthProfileHandler(userRepo, eventPub, txManager)
	updateProfileHandler := command.NewUpdateProfileHandler(userRepo, eventPub, txManager)
	logPeriodicMetricsHandler := command.NewLogPeriodicMetricsHandler(userRepo, eventPub, txManager)
	reportInjuryHandler := command.NewReportInjuryHandler(userRepo, eventPub, txManager)
	recoverInjuryHandler := command.NewRecoverInjuryHandler(userRepo, eventPub, txManager)
	getProfileHandler := query.NewGetProfileHandler(userRepo)
	getBodyMetricsHistoryHandler := query.NewGetBodyMetricsHistoryHandler(userRepo)
	getInjuryHistoryHandler := query.NewGetInjuryHistoryHandler(userRepo)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	kafkaBrokersStr := os.Getenv("PROFILE_KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	var kafkaPub *profileKafka.Publisher
	if deps.KafkaRegistry != nil {
		writer, err := deps.KafkaRegistry.GetWriter("profile", kafkaBrokers)
		if err != nil {
			cancelWorkers()
			return nil, fmt.Errorf("get profile kafka writer: %w", err)
		}
		kafkaPub = profileKafka.NewPublisher(writer)
		outboxWorker := worker.NewOutboxWorker(outboxRepo, kafkaPub, 2*time.Second)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC RECOVERED in Profile Outbox worker: %v", r)
				}
			}()
			outboxWorker.Start(workerCtx)
		}()

		// Start UserRegistered Kafka Consumer to automatically create blank profiles
		reader, err := deps.KafkaRegistry.GetReader("profile-user-registered-consumer", "auth.events", kafkaBrokers)
		if err == nil {
			consumer := worker.NewUserRegisteredConsumer(reader, userRepo, outboxLogRepo, txManager)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED in UserRegistered Consumer: %v", r)
					}
				}()
				consumer.Start(workerCtx)
			}()
		} else {
			log.Printf("Warning: failed to get kafka reader for auth.events: %v", err)
		}
	}

	shutdown := func() {
		log.Println("Shutting down Profile Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		if kafkaPub != nil {
			_ = kafkaPub.Close()
		}
		log.Println("Profile Bounded Context background workers stopped.")
	}

	grpcHandler := grpcProfile.NewGRPCHandler(
		saveHealthProfileHandler,
		updateProfileHandler,
		logPeriodicMetricsHandler,
		reportInjuryHandler,
		recoverInjuryHandler,
		getProfileHandler,
		getBodyMetricsHistoryHandler,
		getInjuryHistoryHandler,
	)
	profilev1service.RegisterProfileServiceServer(deps.GRPCServer, grpcHandler)

	log.Println("Profile Bounded Context initialized successfully.")
	return shutdown, nil
}

func RegisterGateway(
	ctx context.Context,
	mux *runtime.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	err := profilev1service.RegisterProfileServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts)
	if err != nil {
		return fmt.Errorf("register profile service gateway handler: %w", err)
	}
	return nil
}
