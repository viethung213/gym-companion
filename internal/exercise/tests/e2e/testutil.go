//go:build e2e

package e2e

import (
	"context"
	"net"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise"
	exerciseGRPC "github.com/viethung213/gym-companion/internal/exercise/transport/grpc"
	exercisesvc "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type TestSuite struct {
	DB         *gorm.DB
	GRPCConn   *grpc.ClientConn
	Client     exercisesvc.ExerciseServiceClient
	Server     *exerciseGRPC.ExerciseServer
	StopServer func()
}

// truncateTables empties the exercise tables; the schema comes from the
// embedded migrations that GetPool applies.
func truncateTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE exercise.exercises, exercise.outbox CASCADE").Error; err != nil {
		t.Fatalf("truncate exercise tables: %v", err)
	}
}

func SetupE2ESuite(t *testing.T) *TestSuite {
	t.Helper()

	sqlDB, err := database.GetRegistry().GetPool("exercise")
	if err != nil {
		t.Fatalf("Failed to initialize exercise DB pool: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	truncateTables(t, db)

	// Start local gRPC server on a random free port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on random port: %v", err)
	}

	testAuthInterceptor := func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-user-id"); len(vals) > 0 && vals[0] != "" {
				ctx = context.WithValue(ctx, middleware.UserIDKey, vals[0])
			}
			if vals := md.Get("x-user-role"); len(vals) > 0 && vals[0] != "" {
				ctx = context.WithValue(ctx, middleware.UserRoleKey, vals[0])
			}
		}
		return handler(ctx, req)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(testAuthInterceptor))
	ctx, cancel := context.WithCancel(context.Background())

	grpcHandler, cleanup, err := exercise.Initialize(ctx, exercise.ModuleDeps{
		DB:            sqlDB,
		KafkaRegistry: sharedKafka.GetRegistry(),
	})
	if err != nil {
		cancel()
		t.Fatalf("Failed to initialize exercise module: %v", err)
	}

	exercisesvc.RegisterExerciseServiceServer(grpcServer, grpcHandler)

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		grpcServer.Stop()
		cleanup()
		cancel()
		t.Fatalf("Failed to connect to gRPC test server: %v", err)
	}

	client := exercisesvc.NewExerciseServiceClient(conn)

	stopServer := func() {
		truncateTables(t, db)
		_ = conn.Close()
		grpcServer.GracefulStop()
		cleanup()
		cancel()
	}

	return &TestSuite{
		DB:         db,
		GRPCConn:   conn,
		Client:     client,
		Server:     grpcHandler,
		StopServer: stopServer,
	}
}
