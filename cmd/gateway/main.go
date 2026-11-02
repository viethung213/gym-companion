// Package main provides the entrypoint for the Custom API Gateway.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway"
	"github.com/viethung213/gym-companion/internal/gateway/admin"
	"github.com/viethung213/gym-companion/internal/gateway/config"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal gateway error: %v", err)
	}
}

func run() error {
	cfg := config.LoadStartupConfig()

	log.Printf("🚀 [API Gateway] Initializing on port :%s (Env: %s)", cfg.Port, cfg.AppEnv)
	log.Printf("📄 [API Gateway] Route config: %s", cfg.ConfigPath)
	log.Printf("⏱️ [API Gateway] Token TTL: %v | Watch Interval: %v", cfg.TokenTTL, cfg.WatchInterval)
	log.Printf("🔒 [API Gateway] Enforce TLS (HTTPS): %v", cfg.EnforceTLS)

	// Gateway automatically discovers Auth Service upstream from the route config!
	gw, err := gateway.NewGateway(gateway.Options{
		ConfigPath:      cfg.ConfigPath,
		InternalSecret:  cfg.InternalSecret,
		TokenTTL:        cfg.TokenTTL,
		WatchInterval:   cfg.WatchInterval,
		EnforceTLS:      cfg.EnforceTLS,
		TraceBufferSize: cfg.TraceBufferSize,
	})
	if err != nil {
		return fmt.Errorf("create gateway instance: %w", err)
	}
	defer gw.Close()

	// Wrap with h2c for HTTP/2 cleartext streaming & gRPC support
	h2cHandler := h2c.NewHandler(gw.Handler(), &http2.Server{})

	gatewayServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h2cHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	var adminServer *admin.Server
	if cfg.AdminEnabled {
		// Adapter lấy trực tiếp Telemetry, ConfigManager và Discovery từ Gateway
		adminInspector := admin.NewAdapter(gw.Telemetry(), gw.ConfigManager(), gw)
		adminServer = admin.NewServer(cfg.AdminPort, cfg.AdminKey, adminInspector)
	}

	errChan := make(chan error, 2)

	go func() {
		log.Printf("✅ [API Gateway] Listening and serving at http://localhost:%s (Stateless, Zero-DB, Auto-Auth Discovery)", cfg.Port)
		if err := gatewayServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("gateway server: %w", err)
		}
	}()

	if adminServer != nil {
		go func() {
			if err := adminServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("admin server: %w", err)
			}
		}()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		log.Printf("🛑 [API Gateway] Received signal %v, initiating graceful shutdown...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if adminServer != nil {
		_ = adminServer.Shutdown(ctx)
	}
	if err := gatewayServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("gateway server shutdown: %w", err)
	}

	log.Println("👋 [API Gateway] Graceful shutdown complete")
	return nil
}
