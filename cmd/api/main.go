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

	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/shared/database"
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

	// Initialize Database Registry & connection pool for auth module
	dbRegistry := database.GetRegistry()
	defer dbRegistry.CloseAll()

	db, err := dbRegistry.GetPool("auth")
	if err != nil {
		return fmt.Errorf("initialize auth database pool: %w", err)
	}
	log.Println("Initialized isolated Auth Database Pool successfully.")

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
	})
	if err != nil {
		return fmt.Errorf("initialize auth module: %w", err)
	}
	defer shutdown()

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

	// Delegate gRPC-Gateway setup to Auth module
	err = auth.RegisterGateway(ctx, mux, ":"+grpcPort, opts)
	if err != nil {
		return fmt.Errorf("register auth gateway: %w", err)
	}

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
