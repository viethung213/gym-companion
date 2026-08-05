// Package main provides the entrypoint for the API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	connectcors "connectrpc.com/cors"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/coaching"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	coachingpersistence "github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/exercise"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	workoutexecution "github.com/viethung213/gym-companion/internal/workout_execution"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err) //nolint:forbidigo // Only allowed once in main
	}
}

func run() error {
	loadEnvFile()

	httpPort := os.Getenv("APP_PORT")
	if httpPort == "" {
		httpPort = "1010"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9090"
	}

	// Initialize Database Registry & connection pools
	dbRegistry := database.GetRegistry()
	defer dbRegistry.CloseAll()

	// Initialize Kafka Registry
	kafkaRegistry := sharedKafka.GetRegistry()
	defer kafkaRegistry.CloseAll()

	db, err := dbRegistry.GetPool("auth")
	if err != nil {
		return fmt.Errorf("initialize auth database pool: %w", err)
	}
	log.Println("Initialized isolated Auth Database Pool successfully.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run embedded SQL migrations (idempotent CREATE TABLE IF NOT EXISTS / ON CONFLICT)
	if migErr := database.RunAutoMigrations(ctx, db); migErr != nil {
		return fmt.Errorf("run database auto migrations: %w", migErr)
	}

	exerciseDB, err := dbRegistry.GetPool("exercise")
	if err != nil {
		return fmt.Errorf("initialize exercise database pool: %w", err)
	}
	log.Println("Initialized isolated Exercise Database Pool successfully.")

	workoutDB, err := dbRegistry.GetPool("workout_execution")
	if err != nil {
		log.Println("Warning: workout_execution database pool not found, falling back to auth pool.")
		workoutDB = db
	} else {
		log.Println("Initialized isolated Workout Execution Database Pool successfully.")
	}

	// Listen on gRPC port
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		return fmt.Errorf("grpc listen on port %s: %w", grpcPort, err)
	}

	lazyKP := &lazyKeyProvider{}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryRecoveryInterceptor(),
			middleware.UnaryLoggingInterceptor(),
			middleware.UnaryAuthInterceptor(lazyKP),
			middleware.UnaryRateLimitInterceptor(),
		),
	)
	log.Printf("Starting gRPC server on port %s...\n", grpcPort)

	// HTTP mux created early so modules can mount ConnectRPC routes during initialization
	mux := http.NewServeMux()

	// Initialize Auth Module
	shutdown, err := auth.Initialize(ctx, auth.ModuleDeps{
		DB:                db,
		GRPCServer:        grpcServer,
		ConnectMux:        mux,
		AssignKeyProvider: lazyKP.Set,
		KafkaRegistry:     kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize auth module: %w", err)
	}
	defer shutdown()

	// Initialize Exercise Module
	shutdownExercise, err := exercise.Initialize(ctx, exercise.ModuleDeps{
		DB:            exerciseDB,
		GRPCServer:    grpcServer,
		ConnectMux:    mux,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize exercise module: %w", err)
	}
	defer shutdownExercise()

	// Initialize Workout Execution Module
	shutdownWorkout, err := workoutexecution.Initialize(ctx, workoutexecution.ModuleDeps{
		DB:            workoutDB,
		GRPCServer:    grpcServer,
		ConnectMux:    mux,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize workout execution module: %w", err)
	}
	defer shutdownWorkout()

	// Initialize Coaching Module.
	// TODO(#197): all three readers are mocks; coaching runs on fixture data.
	coachAgent, shutdownCoaching, err := coaching.Initialize(ctx, &coaching.ModuleDeps{
		GRPCServer:    grpcServer,
		KafkaRegistry: kafkaRegistry,
		ProfileReader: &adapters.MockUserProfileReader{},
		SessionReader: &adapters.MockWorkoutSessionReader{},
		CatalogReader: &adapters.MockExerciseCatalogReader{},
		IDGenerator:   coachingpersistence.UUIDGenerator{},
	})
	if err != nil {
		return fmt.Errorf("initialize coaching module: %w", err)
	}
	defer shutdownCoaching()

	errChan := make(chan error, 2)
	go func() {
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			errChan <- fmt.Errorf("grpc server serve: %w", serveErr)
		}
	}()

	// 2. Start HTTP Server (gRPC-Gateway + ConnectRPC)
	log.Printf("Starting HTTP API gateway server on port %s...\n", httpPort)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Register Coaching HTTP Handler FIRST (before gwmux catch-all)
	coachingHandler := coaching.NewCoachingHandler(coachAgent)
	coaching.RegisterHandlers(mux, coachingHandler)

	// Setup shared gRPC-Gateway multiplexer
	gwmux := runtime.NewServeMux()

	err = auth.RegisterGateway(ctx, gwmux, ":"+grpcPort, opts)
	if err != nil {
		return fmt.Errorf("register auth gateway: %w", err)
	}

	err = exercise.RegisterGateway(ctx, gwmux, ":"+grpcPort, opts)
	if err != nil {
		return fmt.Errorf("register exercise gateway: %w", err)
	}

	err = workoutexecution.RegisterGateway(ctx, gwmux, ":"+grpcPort, opts)
	if err != nil {
		return fmt.Errorf("register workout execution gateway: %w", err)
	}

	// Register gwmux LAST (catch-all for remaining routes)
	mux.Handle("/", gwmux)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(mux)
	h2cHandler := h2c.NewHandler(corsHandler, &http2.Server{})

	go func() {
		if err := http.ListenAndServe(":"+httpPort, h2cHandler); err != nil {
			errChan <- fmt.Errorf("http server listen and serve: %w", err)
		}
	}()

	return <-errChan
}

type lazyKeyProvider struct {
	mu sync.RWMutex
	kp middleware.KeyProvider
}

var _ middleware.KeyProvider = (*lazyKeyProvider)(nil)

func (l *lazyKeyProvider) GetPublicKeyPEM(ctx context.Context, kid string) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.kp == nil {
		return "", errors.New("key provider not initialized")
	}
	return l.kp.GetPublicKeyPEM(ctx, kid)
}

func (l *lazyKeyProvider) Set(kp middleware.KeyProvider) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.kp = kp
}

func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && os.Getenv(parts[0]) == "" {
			os.Setenv(parts[0], parts[1])
		}
	}
}
