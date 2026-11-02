// Package main provides the entrypoint for the AI & Notification service.
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
	"github.com/viethung213/gym-companion/internal/coaching"
	coachingpersistence "github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/notification"
	"github.com/viethung213/gym-companion/internal/nutrition"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"github.com/viethung213/gym-companion/internal/shared/discovery"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	sharedRedis "github.com/viethung213/gym-companion/internal/shared/redis"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err) //nolint:forbidigo // Only allowed once in main
	}
}

func run() error {
	loadEnvFiles(".env.ai", ".env")

	dir, _ := os.Getwd()
	aiProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if aiProvider == "" {
		aiProvider = "groq"
	}
	groqKey := os.Getenv("GROQ_API_KEY")
	groqModel := os.Getenv("GROQ_MODEL")
	if groqModel == "" {
		groqModel = "openai/gpt-oss-120b"
	}
	coachingKey := os.Getenv("GOOGLE_API_KEY_COACHING")
	nutritionKey := os.Getenv("GOOGLE_API_KEY_NUTRITION")

	log.Printf("📂 [AI Service] Working Directory: %s", dir)
	log.Printf("🤖 [AI Provider] Active: %s (Groq Model: %s)", aiProvider, groqModel)
	log.Printf("🔑 [Env Check] GROQ_API_KEY: %s", maskKey(groqKey))
	log.Printf("🔑 [Env Check] GOOGLE_API_KEY_COACHING: %s", maskKey(coachingKey))
	log.Printf("🔑 [Env Check] GOOGLE_API_KEY_NUTRITION: %s", maskKey(nutritionKey))

	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "8082"
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

	coachingDB, err := dbRegistry.GetPool("coaching")
	if err != nil {
		return fmt.Errorf("initialize coaching database pool: %w", err)
	}
	log.Println("Initialized isolated Coaching Database Pool successfully.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Explicitly ensure AI SQL migrations are executed
	if migErr := database.RunAutoMigrations(ctx, coachingDB, "ai"); migErr != nil {
		return fmt.Errorf("run ai database auto migrations: %w", migErr)
	}

	nutritionDB, err := dbRegistry.GetPool("nutrition")
	if err != nil {
		return fmt.Errorf("initialize nutrition database pool: %w", err)
	}
	log.Println("Initialized isolated Nutrition Database Pool successfully.")

	notificationDB, err := dbRegistry.GetPool("notification")
	if err != nil {
		return fmt.Errorf("initialize notification database pool: %w", err)
	}
	log.Println("Initialized isolated Notification Database Pool successfully.")

	internalSecret := os.Getenv("INTERNAL_SECRET")
	if internalSecret == "" {
		internalSecret = "gym-companion-internal-secret-2026-key"
	}

	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8000"
	}
	log.Printf("🌐 [AI Service] Gateway Upstream URL for Core RPC: %s", gatewayURL)

	// Initialize Coaching Module
	coachGRPCHandler, _, shutdownCoaching, err := coaching.Initialize(ctx, &coaching.ModuleDeps{
		DB:             coachingDB,
		KafkaRegistry:  kafkaRegistry,
		IDGenerator:    coachingpersistence.UUIDGenerator{},
		GatewayURL:     gatewayURL,
		InternalSecret: internalSecret,
	})
	if err != nil {
		return fmt.Errorf("initialize coaching module: %w", err)
	}
	defer shutdownCoaching()

	// Initialize Nutrition Module
	nutritionGRPCHandler, shutdownNutrition, err := nutrition.Initialize(ctx, nutrition.ModuleDeps{
		DB:             nutritionDB,
		KafkaRegistry:  kafkaRegistry,
		GatewayURL:     gatewayURL,
		InternalSecret: internalSecret,
	})
	if err != nil {
		return fmt.Errorf("initialize nutrition module: %w", err)
	}
	defer shutdownNutrition()

	// Initialize Notification Module
	notificationGRPCHandler, shutdownNotification, err := notification.Initialize(ctx, notification.ModuleDeps{
		DB:            notificationDB,
		KafkaRegistry: kafkaRegistry,
	})
	if err != nil {
		return fmt.Errorf("initialize notification module: %w", err)
	}
	defer shutdownNotification()

	// Start HTTP Server with ConnectRPC, h2c, and CORS
	log.Printf("🚀 Starting AI Service (ConnectRPC + gRPC over h2c) on port %s...\n", appPort)
	mux := http.NewServeMux()

	connectInterceptors := connect.WithInterceptors(
		middleware.NewConnectRecoveryInterceptor(),
		middleware.NewConnectLoggingInterceptor(),
		middleware.NewConnectErrorMappingInterceptor(),
		middleware.NewConnectInternalAuthInterceptor(internalSecret),
	)

	// Mount AI ConnectRPC Service Handlers
	prefixes := []string{
		coaching.RegisterConnectHandler(mux, coachGRPCHandler, connectInterceptors),
		nutrition.RegisterConnectHandler(mux, nutritionGRPCHandler, connectInterceptors),
		notification.RegisterConnectHandler(mux, notificationGRPCHandler, connectInterceptors),
	}

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK (AI Service)"))
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

	// Run background seeder tasks for AI
	go func() {
		seedCtx := context.Background()
		log.Println("🌱 Triggering background seeding for Food catalog...")
		if seedErr := seeder.SeedFoodItems(seedCtx, nutritionDB, nutritionGRPCHandler); seedErr != nil {
			log.Printf("⚠️ Background seeding food catalog error: %v", seedErr)
		}
	}()

	// Auto-register with API Gateway via Service Discovery
	gatewayAdminURL := os.Getenv("GATEWAY_ADMIN_URL")
	gatewayAdminKey := os.Getenv("GATEWAY_ADMIN_KEY")
	if gatewayAdminKey == "" {
		gatewayAdminKey = "gym-companion-admin-secret-2026"
	}

	serviceID := os.Getenv("SERVICE_ID")
	if serviceID == "" {
		serviceID = "ai-notification-service"
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
		log.Printf("🛑 [AI Service] Received signal %v, initiating graceful shutdown...", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	log.Println("👋 [AI Service] Graceful shutdown complete")
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

func maskKey(k string) string {
	if k == "" {
		return "<EMPTY>"
	}
	if len(k) <= 8 {
		return k[:2] + "****"
	}
	return k[:6] + "..." + k[len(k)-4:] + fmt.Sprintf(" (len=%d)", len(k))
}
