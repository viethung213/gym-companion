// Package main provides the entrypoint for the API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/coaching"
	coachingpersistence "github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/exercise"
	"github.com/viethung213/gym-companion/internal/notification"
	"github.com/viethung213/gym-companion/internal/nutrition"
	"github.com/viethung213/gym-companion/internal/profile"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	workoutexecution "github.com/viethung213/gym-companion/internal/workout_execution"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err) //nolint:forbidigo // Only allowed once in main
	}
}

func run() error {
	loadEnvFile()

	dir, _ := os.Getwd()
	coachingKey := os.Getenv("GOOGLE_API_KEY_COACHING")
	nutritionKey := os.Getenv("GOOGLE_API_KEY_NUTRITION")

	log.Printf("📂 Working Directory: %s", dir)
	log.Printf("🔑 [Env Check] GOOGLE_API_KEY_COACHING: %s", maskKey(coachingKey))
	log.Printf("🔑 [Env Check] GOOGLE_API_KEY_NUTRITION: %s", maskKey(nutritionKey))

	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "8080"
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

	lazyKP := &lazyKeyProvider{}

	// Initialize Auth Module
	authGRPCHandler, shutdownAuth, err := auth.Initialize(ctx, auth.ModuleDeps{
		DB:                db,
		AssignKeyProvider: lazyKP.Set,
		KafkaRegistry:     kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize auth module: %w", err)
	}
	defer shutdownAuth()

	// Initialize Exercise Module
	exerciseServer, shutdownExercise, err := exercise.Initialize(ctx, exercise.ModuleDeps{
		DB:            exerciseDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize exercise module: %w", err)
	}
	defer shutdownExercise()

	// Initialize Workout Execution Module
	workoutServer, shutdownWorkout, err := workoutexecution.Initialize(ctx, workoutexecution.ModuleDeps{
		DB:            workoutDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize workout execution module: %w", err)
	}
	defer shutdownWorkout()

	// Initialize Nutrition Module
	nutritionDB, err := dbRegistry.GetPool("nutrition")
	if err != nil {
		log.Println("Warning: nutrition database pool not found, falling back to auth pool.")
		nutritionDB = db
	}
	nutritionGRPCHandler, shutdownNutrition, err := nutrition.Initialize(ctx, nutrition.ModuleDeps{
		DB:            nutritionDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize nutrition module: %w", err)
	}
	defer shutdownNutrition()

	// Initialize Profile Module
	profileDB, err := dbRegistry.GetPool("profile")
	if err != nil {
		log.Println("Warning: profile database pool not found, falling back to auth pool.")
		profileDB = db
	}
	profileGRPCHandler, shutdownProfile, err := profile.Initialize(ctx, profile.ModuleDeps{
		DB:            profileDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize profile module: %w", err)
	}
	defer shutdownProfile()

	// Initialize Notification Module
	notificationDB, err := dbRegistry.GetPool("notification")
	if err != nil {
		log.Println("Warning: notification database pool not found, falling back to auth pool.")
		notificationDB = db
	}
	notificationGRPCHandler, shutdownNotification, err := notification.Initialize(ctx, notification.ModuleDeps{
		DB:            notificationDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize notification module: %w", err)
	}
	defer shutdownNotification()

	// Initialize Coaching Module.
	coachingDB, err := dbRegistry.GetPool("coaching")
	if err != nil {
		log.Println("Warning: coaching database pool not found, falling back to auth pool.")
		coachingDB = db
	}
	coachAgent, shutdownCoaching, err := coaching.Initialize(ctx, &coaching.ModuleDeps{
		DB:            coachingDB,
		KafkaRegistry: kafkaRegistry,
		IDGenerator:   coachingpersistence.UUIDGenerator{},
	})
	if err != nil {
		return fmt.Errorf("initialize coaching module: %w", err)
	}
	defer shutdownCoaching()

	// 2. Start Unified HTTP Server with ConnectRPC, h2c, and CORS on Port 8080
	log.Printf("🚀 Starting Unified API Server (ConnectRPC + gRPC over h2c) on port %s...\n", appPort)
	mux := http.NewServeMux()

	connectInterceptors := connect.WithInterceptors(
		middleware.NewConnectRecoveryInterceptor(),
		middleware.NewConnectLoggingInterceptor(),
		middleware.NewConnectAuthInterceptor(lazyKP),
		middleware.NewConnectRateLimitInterceptor(),
	)

	// Mount ConnectRPC Service Handlers directly onto HTTP multiplexer
	auth.RegisterConnectHandler(mux, authGRPCHandler, connectInterceptors)
	exercise.RegisterConnectHandler(mux, exerciseServer, connectInterceptors)
	workoutexecution.RegisterConnectHandler(mux, workoutServer, connectInterceptors)
	nutrition.RegisterConnectHandler(mux, nutritionGRPCHandler, connectInterceptors)
	profile.RegisterConnectHandler(mux, profileGRPCHandler, connectInterceptors)
	notification.RegisterConnectHandler(mux, notificationGRPCHandler, connectInterceptors)

	// Register Coaching REST HTTP Handler
	coachingHandler := coaching.NewCoachingHandler(coachAgent)
	coaching.RegisterHandlers(mux, coachingHandler)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap multiplexer with CORS middleware for browser clients
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept", "Content-Type", "Content-Length", "Accept-Encoding",
			"Authorization", "Connect-Protocol-Version", "Connect-Timeout-Ms",
			"Connect-Accept-Encoding", "Connect-Content-Encoding", "X-User-Id", "X-GRPC-Web",
		},
		ExposedHeaders:   []string{"Content-Type", "Content-Length", "Connect-Content-Encoding", "Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin"},
		AllowCredentials: true,
		MaxAge:           7200,
	}).Handler(mux)

	// Wrap with h2c for unencrypted HTTP/2 support (allows gRPC CLI & standard gRPC clients over HTTP/2)
	h2cHandler := h2c.NewHandler(corsHandler, &http2.Server{})

	server := &http.Server{
		Addr:    ":" + appPort,
		Handler: h2cHandler,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if key != "" && os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func maskKey(k string) string {
	if len(k) == 0 {
		return "<EMPTY>"
	}
	if len(k) <= 8 {
		return k[:2] + "****"
	}
	return k[:6] + "..." + k[len(k)-4:] + fmt.Sprintf(" (len=%d)", len(k))
}
