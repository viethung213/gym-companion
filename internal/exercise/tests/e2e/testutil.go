//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise"
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

type E2ETestSuite struct {
	DB         *gorm.DB
	GRPCConn   *grpc.ClientConn
	Client     exercisesvc.ExerciseServiceClient
	StopServer func()
}

func ensureTablesExist(db *gorm.DB) {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS exercise`,
		`CREATE TABLE IF NOT EXISTS exercise.body_parts (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS exercise.equipments (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS exercise.muscles (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS exercise.tags (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS exercise.exercises (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id),
			equipment_id VARCHAR(255) NOT NULL REFERENCES exercise.equipments(id),
			target_muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id),
			instructions TEXT,
			thumbnail_url VARCHAR(1024),
			media_url VARCHAR(1024),
			video_url VARCHAR(1024),
			difficulty VARCHAR(50) DEFAULT 'Beginner',
			default_rest_seconds INT DEFAULT 60,
			status VARCHAR(50) DEFAULT 'DRAFT',
			archived_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT chk_exercises_status CHECK (
				status IN ('DRAFT', 'PENDING_APPROVAL', 'ACTIVE', 'ARCHIVED')
			)
		)`,
		`CREATE TABLE IF NOT EXISTS exercise.exercise_secondary_muscles (
			exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
			muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id) ON DELETE CASCADE,
			PRIMARY KEY (exercise_id, muscle_id)
		)`,
		`CREATE TABLE IF NOT EXISTS exercise.exercise_tags (
			exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
			tag_id VARCHAR(255) NOT NULL REFERENCES exercise.tags(id) ON DELETE CASCADE,
			PRIMARY KEY (exercise_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS exercise.outbox (
			id UUID PRIMARY KEY,
			event_id UUID NOT NULL UNIQUE,
			event_type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			partition_key VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			published BOOLEAN DEFAULT FALSE,
			published_at TIMESTAMP WITH TIME ZONE
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			panic(fmt.Sprintf("Failed to run migration statement %q: %v", stmt, err))
		}
	}
}

func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE exercise.exercise_tags CASCADE")
	db.Exec("TRUNCATE TABLE exercise.exercise_secondary_muscles CASCADE")
	db.Exec("TRUNCATE TABLE exercise.exercises CASCADE")
	db.Exec("TRUNCATE TABLE exercise.outbox CASCADE")
	db.Exec("TRUNCATE TABLE exercise.tags CASCADE")
	db.Exec("TRUNCATE TABLE exercise.muscles CASCADE")
	db.Exec("TRUNCATE TABLE exercise.equipments CASCADE")
	db.Exec("TRUNCATE TABLE exercise.body_parts CASCADE")
}

func seedMetadata(db *gorm.DB) {
	db.Exec(`INSERT INTO exercise.body_parts (id, name) VALUES ('legs', 'Legs') ON CONFLICT (id) DO NOTHING`)
	db.Exec(`INSERT INTO exercise.equipments (id, name) VALUES ('barbell', 'Barbell') ON CONFLICT (id) DO NOTHING`)
	db.Exec(`INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('quads', 'Quads', 'legs') ON CONFLICT (id) DO NOTHING`)
	db.Exec(`INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('glutes', 'Glutes', 'legs') ON CONFLICT (id) DO NOTHING`)
	db.Exec(`INSERT INTO exercise.tags (id, name) VALUES ('strength', 'Strength') ON CONFLICT (id) DO NOTHING`)
}

func SetupE2ESuite(t *testing.T) *E2ETestSuite {
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

	// Ensure tables exist & clean initial state
	ensureTablesExist(db)
	truncateTables(db)
	seedMetadata(db)

	// Start local gRPC server on a random free port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on random port: %v", err)
	}

	testAuthInterceptor := func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
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

	cleanup, err := exercise.Initialize(ctx, exercise.ModuleDeps{
		DB:            sqlDB,
		GRPCServer:    grpcServer,
		KafkaRegistry: sharedKafka.GetRegistry(),
	})
	if err != nil {
		cancel()
		t.Fatalf("Failed to initialize exercise module: %v", err)
	}

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
		truncateTables(db)
		_ = conn.Close()
		grpcServer.GracefulStop()
		cleanup()
		cancel()
	}

	return &E2ETestSuite{
		DB:         db,
		GRPCConn:   conn,
		Client:     client,
		StopServer: stopServer,
	}
}
