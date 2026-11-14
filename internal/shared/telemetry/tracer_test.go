package telemetry_test

import (
	"context"
	"os"
	"testing"

	"github.com/viethung213/gym-companion/internal/shared/telemetry"
)

func TestInitTracer_DefaultEnabledWithoutEndpoint(t *testing.T) {
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origTracesEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	origEnabled := os.Getenv("OTEL_TRACES_ENABLED")
	origAppEnv := os.Getenv("APP_ENV")
	defer func() {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		_ = os.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", origTracesEndpoint)
		_ = os.Setenv("OTEL_TRACES_ENABLED", origEnabled)
		_ = os.Setenv("APP_ENV", origAppEnv)
	}()

	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	_ = os.Unsetenv("OTEL_TRACES_ENABLED")
	_ = os.Setenv("APP_ENV", "development")

	shutdown, err := telemetry.InitTracer(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("expected no error when endpoint is omitted (defaults to localhost:4317 via gRPC), got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}
}

func TestInitTracer_DisabledExplicitly(t *testing.T) {
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origEnabled := os.Getenv("OTEL_TRACES_ENABLED")
	defer func() {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		_ = os.Setenv("OTEL_TRACES_ENABLED", origEnabled)
	}()

	_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")
	_ = os.Setenv("OTEL_TRACES_ENABLED", "false")

	shutdown, err := telemetry.InitTracer(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("expected no error when traces disabled explicitly, got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}
}

func TestInitTracer_WithEndpoint(t *testing.T) {
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origEnabled := os.Getenv("OTEL_TRACES_ENABLED")
	origEnv := os.Getenv("APP_ENV")
	defer func() {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		_ = os.Setenv("OTEL_TRACES_ENABLED", origEnabled)
		_ = os.Setenv("APP_ENV", origEnv)
	}()

	_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	_ = os.Setenv("OTEL_TRACES_ENABLED", "true")
	_ = os.Setenv("APP_ENV", "test")

	shutdown, err := telemetry.InitTracer(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("expected no error initializing tracer with endpoint, got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Shutdown tracer provider
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}
}
