// Package main provides the entrypoint for the Core API service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"github.com/viethung213/gym-companion/internal/auth"
	"github.com/viethung213/gym-companion/internal/exercise"
	"github.com/viethung213/gym-companion/internal/gamification"
	"github.com/viethung213/gym-companion/internal/profile"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"github.com/viethung213/gym-companion/internal/shared/discovery"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	sharedRedis "github.com/viethung213/gym-companion/internal/shared/redis"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"github.com/viethung213/gym-companion/internal/social"
	solomatch "github.com/viethung213/gym-companion/internal/solo_match"
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
	loadEnvFiles(".env.core", ".env")

	dir, _ := os.Getwd()
	log.Printf("📂 [Core Service] Working Directory: %s", dir)

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

	// Initialize Redis Registry
	redisRegistry := sharedRedis.GetRegistry()
	defer redisRegistry.CloseAll()

	redisClient, rErr := redisRegistry.GetClient()
	if rErr != nil {
		log.Printf("Warning: Redis connection failed: %v. Modules requiring Redis may operate in degraded mode.", rErr)
	}

	db, err := dbRegistry.GetPool("auth")
	if err != nil {
		return fmt.Errorf("initialize auth database pool: %w", err)
	}
	log.Println("Initialized isolated Auth Database Pool successfully.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Explicitly ensure core SQL migrations and seeds are executed
	if migErr := database.RunAutoMigrations(ctx, db, "core"); migErr != nil {
		return fmt.Errorf("run core database auto migrations: %w", migErr)
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

	internalSecret := os.Getenv("INTERNAL_SECRET")
	if internalSecret == "" {
		internalSecret = "gym-companion-internal-secret-2026-key"
	}

	// Initialize Auth Module
	authGRPCHandler, shutdownAuth, err := auth.Initialize(ctx, auth.ModuleDeps{
		DB:            db,
		KafkaRegistry: kafkaRegistry,
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
		RedisClient:   redisClient,
	})
	if err != nil {
		return fmt.Errorf("initialize workout execution module: %w", err)
	}
	defer shutdownWorkout()

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

	// Initialize Social Module
	socialDB, err := dbRegistry.GetPool("social")
	if err != nil {
		log.Println("Warning: social database pool not found, falling back to auth pool.")
		socialDB = db
	}
	socialGRPCHandler, shutdownSocial, err := social.Initialize(ctx, social.ModuleDeps{
		DB:            socialDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize social module: %w", err)
	}
	defer shutdownSocial()

	// Initialize Solo Match Module
	soloMatchDB, err := dbRegistry.GetPool("solo_match")
	if err != nil {
		log.Println("Warning: solo_match database pool not found, falling back to auth pool.")
		soloMatchDB = db
	} else {
		log.Println("Initialized isolated Solo Match Database Pool successfully.")
	}
	soloMatchGRPCHandler, shutdownSoloMatch, err := solomatch.Initialize(ctx, solomatch.ModuleDeps{
		DB:             soloMatchDB,
		RedisClient:    redisClient,
		KafkaRegistry:  kafkaRegistry,
		ExerciseServer: exerciseServer,
	})
	if err != nil {
		return fmt.Errorf("initialize solo_match module: %w", err)
	}
	defer shutdownSoloMatch()

	// Initialize Gamification Module
	gamificationDB, err := dbRegistry.GetPool("gamification")
	if err != nil {
		log.Println("Warning: gamification database pool not found, falling back to auth pool.")
		gamificationDB = db
	}
	gamificationGRPCHandler, shutdownGamification, err := gamification.Initialize(ctx, gamification.ModuleDeps{
		DB:            gamificationDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize gamification module: %w", err)
	}
	defer shutdownGamification()

	// Start HTTP Server with ConnectRPC, h2c, and CORS
	log.Printf("🚀 Starting Core Service (ConnectRPC + gRPC over h2c) on port %s...\n", appPort)
	mux := http.NewServeMux()

	connectInterceptors := connect.WithInterceptors(
		middleware.NewConnectRecoveryInterceptor(),
		middleware.NewConnectLoggingInterceptor(),
		middleware.NewConnectErrorMappingInterceptor(),
		middleware.NewConnectInternalAuthInterceptor(internalSecret),
	)

	// Mount Core ConnectRPC Service Handlers
	var prefixes []string
	prefixes = append(prefixes,
		auth.RegisterConnectHandler(mux, authGRPCHandler, connectInterceptors),
		exercise.RegisterConnectHandler(mux, exerciseServer, connectInterceptors),
	)
	prefixes = append(prefixes, workoutexecution.RegisterConnectHandler(mux, workoutServer, connectInterceptors)...)
	prefixes = append(prefixes,
		profile.RegisterConnectHandler(mux, profileGRPCHandler, connectInterceptors),
		social.RegisterConnectHandler(mux, socialGRPCHandler, connectInterceptors),
		solomatch.RegisterConnectHandler(mux, soloMatchGRPCHandler, connectInterceptors),
		gamification.RegisterConnectHandler(mux, gamificationGRPCHandler, connectInterceptors),
	)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK (Core Service)"))
	})

	// Wrap multiplexer with CORS middleware
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

	// Wrap with h2c for unencrypted HTTP/2 support
	h2cHandler := h2c.NewHandler(corsHandler, &http2.Server{})

	server := &http.Server{
		Addr:    ":" + appPort,
		Handler: h2cHandler,
	}

	// Run background seeder tasks for Core
	go func() {
		seedCtx := context.Background()
		log.Println("🌱 Triggering background seeding for System Bots and Exercise catalog...")
		if seedErr := seeder.SeedSystemBots(seedCtx, db, authGRPCHandler); seedErr != nil {
			log.Printf("⚠️ Background seeding system bots error: %v", seedErr)
		}
		if seedErr := seeder.SeedExercises(seedCtx, exerciseDB, exerciseServer); seedErr != nil {
			log.Printf("⚠️ Background seeding exercise catalog error: %v", seedErr)
		}
	}()

	// Auto-register with API Gateway via Service Discovery
	gatewayAdminURL := os.Getenv("GATEWAY_ADMIN_URL")
	if gatewayAdminURL == "" {
		gatewayAdminURL = "http://localhost:8001"
	}
	gatewayAdminKey := os.Getenv("GATEWAY_ADMIN_KEY")
	if gatewayAdminKey == "" {
		gatewayAdminKey = "gym-companion-admin-secret-2026"
	}

	serviceID := os.Getenv("SERVICE_ID")
	if serviceID == "" {
		serviceID = "core-fitness-service"
	}

	registrar := discovery.NewRegistrar(&discovery.Config{
		GatewayAdminURL: gatewayAdminURL,
		AdminKey:        gatewayAdminKey,
		ServiceID:       serviceID,
		ServicePort:     appPort,
		Prefixes:        prefixes,
	})
	registrar.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = registrar.Stop(shutdownCtx)
	}()

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("http server listen and serve: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		log.Printf("🛑 [Core Service] Received signal %v, initiating graceful shutdown...", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	log.Println("👋 [Core Service] Graceful shutdown complete")
	return nil
}

func loadEnvFiles(files ...string) {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		return
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
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
}
