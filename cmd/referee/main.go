package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/cors"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
	"github.com/viethung213/gym-companion/internal/workout_execution/transport/webtransport"
)

func main() {
	log.Println("🚀 Starting Standalone AI Camera Referee & WebTransport Server...")

	// 1. Initialize Authoritative Referee Service with 12 Embedded Exercise Rules & Dialogues
	ruleProvider := rule.NewCachedRuleProvider(nil, nil)
	evaluator := service.NewDefaultRuleEvaluator()
	workoutService := command.NewWorkoutTrackingService(ruleProvider, evaluator)
	soloService := command.NewSoloTrackingService(ruleProvider, evaluator, nil, nil, nil, nil)

	wtAddr := os.Getenv("WT_ADDR")
	if wtAddr == "" {
		wtAddr = "0.0.0.0:8443"
	}
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = "0.0.0.0:8080"
	}

	// 2. Token Validator: accepts JWT or dev tokens
	validator := func(ctx context.Context, token string) (string, string, error) {
		if token == "" {
			return "", "", errors.New("empty token")
		}
		if strings.HasPrefix(token, "mock_") || strings.HasPrefix(token, "dev-") || strings.HasPrefix(token, "user-") || token == "guest" || token == "dev-client" {
			return "dev-athlete-1", "user", nil
		}
		return "authenticated-user", "user", nil
	}

	// 3. Initialize WebTransport HTTP/3 Server
	wtServer, err := webtransport.New(webtransport.Config{
		Addr:           wtAddr,
		Validator:      validator,
		WorkoutService: workoutService,
		Service:        soloService,
		AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000", "https://localhost:3000"},
		IdleTimeout:    45 * time.Second,
	})
	if err != nil {
		log.Fatalf("failed to create webtransport server: %v", err)
	}

	if err := wtServer.Start(); err != nil {
		log.Fatalf("failed to start webtransport server on %s: %v", wtAddr, err)
	}
	log.Printf("✅ WebTransport (HTTP/3 QUIC) listening on %s (UDP)", wtAddr)

	// 4. HTTP & WebSocket Fallback Mux
	mux := http.NewServeMux()
	mux.Handle(webtransport.ConfigPath, wtServer.ConfigHandler())
	mux.Handle(webtransport.WebSocketPath, wtServer.WebSocketHandler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","referee":"ready","exercises":12}`))
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(mux)

	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("✅ HTTP & WebSocket Fallback listening on %s (TCP)", httpAddr)
		log.Printf("   -> Config: http://localhost:8080%s", webtransport.ConfigPath)
		log.Printf("   -> WebSocket: ws://localhost:8080%s", webtransport.WebSocketPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for terminate signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down referee server...")
	_ = wtServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	log.Println("Server gracefully stopped.")
}
