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
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/coaching"
	coachingexercise "github.com/viethung213/gym-companion/internal/coaching/infrastructure/exercise"
	"github.com/viethung213/gym-companion/internal/exercise"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err) //nolint:forbidigo // Only allowed once in main
	}
}

func run() error {
	httpPort := os.Getenv("APP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9090"
	}

	// Initialize Database Registry & connection pool for auth and exercise modules
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

	exerciseDB, err := dbRegistry.GetPool("exercise")
	if err != nil {
		return fmt.Errorf("initialize exercise database pool: %w", err)
	}
	log.Println("Initialized isolated Exercise Database Pool successfully.")

	coachingDB, err := dbRegistry.GetPool("coaching")
	if err != nil {
		return fmt.Errorf("initialize coaching database pool: %w", err)
	}
	log.Println("Initialized isolated Coaching Database Pool successfully.")

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Auth Module (Composition Root bootstrap)
	shutdown, err := auth.Initialize(ctx, auth.ModuleDeps{
		DB:                db,
		GRPCServer:        grpcServer,
		AssignKeyProvider: lazyKP.Set,
		KafkaRegistry:     kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize auth module: %w", err)
	}
	defer shutdown()

	// Initialize Exercise Module
	exerciseRuntime, err := exercise.Initialize(ctx, exercise.ModuleDeps{
		DB:            exerciseDB,
		GRPCServer:    grpcServer,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize exercise module: %w", err)
	}
	defer exerciseRuntime.Shutdown()

	exerciseService, err := coachingexercise.NewAdapter(exerciseRuntime.Catalog)
	if err != nil {
		return fmt.Errorf("initialize coaching exercise service: %w", err)
	}

	// Initialize Coaching Module
	shutdownCoaching, err := coaching.Initialize(ctx, coaching.ModuleDeps{
		DB:              coachingDB,
		GRPCServer:      grpcServer,
		KafkaRegistry:   kafkaRegistry,
		ExerciseService: exerciseService,
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

	// 2. Start HTTP Server (gRPC-Gateway)
	log.Printf("Starting HTTP API gateway server on port %s...\n", httpPort)
	mux := http.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

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

	err = coaching.RegisterGateway(ctx, gwmux, ":"+grpcPort, opts)
	if err != nil {
		return fmt.Errorf("register coaching gateway: %w", err)
	}

	mux.Handle("/", gwmux)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	go func() {
		if err := http.ListenAndServe(":"+httpPort, mux); err != nil {
			errChan <- fmt.Errorf("http server listen and serve: %w", err)
		}
	}()

	return <-errChan
}

type lazyKeyProvider struct {
	mu sync.RWMutex
	kp middleware.KeyProvider
}

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
