package coaching

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	coachingkafka "github.com/viethung213/gym-companion/internal/coaching/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/transport"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/worker"
	coachingsvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	sharedkafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ModuleDeps struct {
	DB               *sql.DB
	GRPCServer       *grpc.Server
	KafkaRegistry    *sharedkafka.Registry
	ExerciseSearcher port.ExerciseSearcher
}

var errRequiredDependencies = errors.New("coaching module dependencies are required")

func Initialize(ctx context.Context, deps ModuleDeps) (func(), error) {
	if deps.DB == nil || deps.GRPCServer == nil || deps.KafkaRegistry == nil || deps.ExerciseSearcher == nil {
		return nil, errRequiredDependencies
	}
	gormDB, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: deps.DB}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wrap coaching database pool: %w", err)
	}
	repository := persistence.NewPostgresRepository(gormDB)
	clock := persistence.SystemClock{}
	ids := persistence.RandomIDGenerator{}
	server := transport.NewCoachingServer(
		command.NewInitiateRoadmapHandler(repository, clock, ids),
		command.NewGenerateWeeklyScheduleHandler(repository, clock, ids),
		command.NewGenerateDailyPlanHandler(repository, deps.ExerciseSearcher, clock, ids),
		query.NewHandler(repository),
	)
	coachingsvc.RegisterCoachingServiceServer(deps.GRPCServer, server)

	brokers := os.Getenv("COACHING_KAFKA_BROKERS")
	if brokers == "" {
		brokers = os.Getenv("KAFKA_BROKERS")
	}
	if brokers == "" {
		brokers = "localhost:9092"
	}
	writer, err := deps.KafkaRegistry.GetWriter("coaching", strings.Split(brokers, ","))
	if err != nil {
		return nil, fmt.Errorf("get coaching kafka writer: %w", err)
	}
	outboxWorker := worker.NewOutboxWorker(repository, coachingkafka.NewPublisher(writer), time.Second)
	workerContext, cancelWorker := context.WithCancel(ctx)
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("PANIC RECOVERED in Coaching Outbox worker: %v", recovered)
			}
		}()
		outboxWorker.Start(workerContext)
	}()

	shutdown := func() {
		log.Println("Shutting down Coaching bounded context")
		cancelWorker()
		waitGroup.Wait()
	}
	log.Println("Coaching bounded context initialized")
	return shutdown, nil
}

func RegisterGateway(
	ctx context.Context,
	mux *runtime.ServeMux,
	grpcEndpoint string,
	opts []grpc.DialOption,
) error {
	if err := coachingsvc.RegisterCoachingServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		return fmt.Errorf("register coaching gateway: %w", err)
	}
	return nil
}
