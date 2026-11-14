// Package main provides the entrypoint for the Custom API Gateway.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway"
	"github.com/viethung213/gym-companion/internal/gateway/admin"
	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal gateway error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	telemetry.InitLogger("fitai-gateway")

	shutdownTracer, err := telemetry.InitTracer(context.Background(), "fitai-gateway")
	if err != nil {
		slog.Warn("Failed to initialize OpenTelemetry tracer for gateway", "error", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownTracer(shutdownCtx)
		}()
	}

	cfg := config.LoadStartupConfig()

	slog.Info("Initializing API Gateway",
		"port", cfg.Port,
		"env", cfg.AppEnv,
		"route_config", cfg.ConfigPath,
		"token_ttl", cfg.TokenTTL.String(),
		"watch_interval", cfg.WatchInterval.String(),
		"enforce_tls", cfg.EnforceTLS,
	)

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
		slog.Info("API Gateway listening and serving",
			"addr", "http://localhost:"+cfg.Port,
			"mode", "Stateless, Zero-DB, Auto-Auth Discovery",
		)
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
		slog.Info("Received signal, initiating graceful shutdown", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if adminServer != nil {
		_ = adminServer.Shutdown(ctx)
	}
	if err := gatewayServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("gateway server shutdown: %w", err)
	}

	slog.Info("API Gateway graceful shutdown complete")
	return nil
}
