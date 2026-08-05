//go:build e2e || integration

package e2e

import (
	"context"
	"net"
	"testing"

	profilev1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	"github.com/viethung213/gym-companion/internal/profile"
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
	Client     profilev1service.ProfileServiceClient
	StopServer func()
}

func ensureTablesExist(db *gorm.DB) {
	db.Exec(`CREATE SCHEMA IF NOT EXISTS profile;`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.users (
		user_id VARCHAR(255) PRIMARY KEY,
		date_of_birth TIMESTAMP WITH TIME ZONE,
		gender VARCHAR(50),
		experience_level VARCHAR(50),
		goals JSONB,
		preferred_workout_times JSONB,
		available_equipment JSONB,
		preferred_muscle_groups JSONB,
		coach_style VARCHAR(50),
		target_weight_kg NUMERIC(10, 2),
		target_body_fat_percent NUMERIC(5, 2),
		completion_rate NUMERIC(5, 2),
		ai_coach_activated BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.body_metrics (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
		weight_kg NUMERIC(10, 2),
		height_cm NUMERIC(10, 2),
		body_fat_percent NUMERIC(5, 2),
		progress_photo_url VARCHAR(1024),
		logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.injuries (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
		muscle_group VARCHAR(100) NOT NULL,
		severity VARCHAR(50) NOT NULL,
		notes TEXT,
		reported_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		recovered_at TIMESTAMP WITH TIME ZONE
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.outbox (
		id VARCHAR(255) PRIMARY KEY,
		event_id VARCHAR(255),
		aggregate_type VARCHAR(255),
		aggregate_id VARCHAR(255),
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255),
		published BOOLEAN DEFAULT FALSE,
		published_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.outbox_log (
		id UUID PRIMARY KEY,
		event_id UUID NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255) NOT NULL,
		processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(50) NOT NULL,
		error_message TEXT
	);`)
}

func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE profile.injuries CASCADE")
	db.Exec("TRUNCATE TABLE profile.body_metrics CASCADE")
	db.Exec("TRUNCATE TABLE profile.users CASCADE")
	db.Exec("TRUNCATE TABLE profile.outbox CASCADE")
	db.Exec("TRUNCATE TABLE profile.outbox_log CASCADE")
}

func SetupE2ESuite(t *testing.T) *TestSuite {
	t.Helper()

	sqlDB, err := database.GetRegistry().GetPool("profile")
	if err != nil {
		t.Fatalf("Failed to initialize profile DB pool from monolith registry: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	ensureTablesExist(db)
	truncateTables(db)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on random port for gRPC: %v", err)
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

	grpcHandler, cleanup, err := profile.Initialize(ctx, profile.ModuleDeps{
		DB:            sqlDB,
		KafkaRegistry: sharedKafka.GetRegistry(),
	})
	if err != nil {
		cancel()
		t.Fatalf("Failed to initialize profile module: %v", err)
	}

	profilev1service.RegisterProfileServiceServer(grpcServer, grpcHandler)

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

	client := profilev1service.NewProfileServiceClient(conn)

	stopServer := func() {
		truncateTables(db)
		_ = conn.Close()
		grpcServer.GracefulStop()
		cleanup()
		cancel()
	}

	return &TestSuite{
		DB:         db,
		GRPCConn:   conn,
		Client:     client,
		StopServer: stopServer,
	}
}
