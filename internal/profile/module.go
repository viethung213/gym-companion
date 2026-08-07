package profile

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
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service/profilev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	profileEvent "github.com/viethung213/gym-companion/internal/profile/infrastructure/event"
	profileKafka "github.com/viethung213/gym-companion/internal/profile/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/worker"
	profileGRPC "github.com/viethung213/gym-companion/internal/profile/transport/grpc"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*profileGRPC.GRPCHandler, func(), error) {
	gormDB, err := gorm.Open(gormPostgres.New(gormPostgres.Config{
		Conn: deps.DB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	userRepo := persistence.NewPostgresUserProfileRepository(gormDB)
	outboxRepo := persistence.NewGormOutboxRepository(gormDB)
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

	var kafkaPub *profileKafka.Publisher
	if deps.KafkaRegistry != nil {
		brokersStr := os.Getenv("KAFKA_BROKERS")
		if brokersStr == "" {
			brokersStr = "localhost:9092"
		}
		brokers := strings.Split(brokersStr, ",")

		writer, wErr := deps.KafkaRegistry.GetWriter("profile-events", brokers)
		if wErr == nil && writer != nil {
			kafkaPub = profileKafka.NewPublisher(writer)
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

	shutdown := func() {
		log.Println("Shutting down Profile Bounded Context background workers...")
		cancelWorkers()
		wg.Wait()
		if kafkaPub != nil {
			_ = kafkaPub.Close()
		}
		log.Println("Profile Bounded Context background workers stopped.")
	}

	grpcHandler := profileGRPC.NewGRPCHandler(
		saveHealthProfileHandler,
		updateProfileHandler,
		logPeriodicMetricsHandler,
		reportInjuryHandler,
		recoverInjuryHandler,
		getProfileHandler,
		getBodyMetricsHistoryHandler,
		getInjuryHistoryHandler,
	)
	log.Println("Profile Bounded Context initialized successfully.")
	return grpcHandler, shutdown, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for the Profile module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *profileGRPC.GRPCHandler,
	opts ...connect.HandlerOption,
) {
	connectHandler := profileGRPC.NewConnectProfileHandler(grpcHandler)
	path, handler := profilev1serviceconnect.NewProfileServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
