package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceHandler_EnrichesWithTraceID(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := telemetry.NewTraceHandler(baseHandler)
	logger := slog.New(handler)

	tracer := tp.Tracer("test-tracer")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	ctx = telemetry.ContextWithModule(ctx, "social")
	logger.InfoContext(ctx, "user action executed", "action", "unfollow")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse json log: %v", err)
	}

	if got, want := parsed["msg"], "user action executed"; got != want {
		t.Errorf("msg got %v, want %v", got, want)
	}

	traceID, ok := parsed["trace_id"].(string)
	if !ok || traceID == "" {
		t.Errorf("expected non-empty trace_id, got %v", parsed["trace_id"])
	}

	if span.SpanContext().TraceID().String() != traceID {
		t.Errorf("trace_id mismatch: got %v, want %v", traceID, span.SpanContext().TraceID().String())
	}

	spanID, ok := parsed["span_id"].(string)
	if !ok || spanID == "" {
		t.Errorf("expected non-empty span_id, got %v", parsed["span_id"])
	}

	if span.SpanContext().SpanID().String() != spanID {
		t.Errorf("span_id mismatch: got %v, want %v", spanID, span.SpanContext().SpanID().String())
	}

	if got, want := parsed["module"], "social"; got != want {
		t.Errorf("module got %v, want %v", got, want)
	}
}

func TestTraceHandler_WithoutTrace(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := telemetry.NewTraceHandler(baseHandler)
	logger := slog.New(handler)

	logger.InfoContext(context.Background(), "no trace message")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse json log: %v", err)
	}

	if _, exists := parsed["trace_id"]; exists {
		t.Errorf("expected no trace_id attribute, found %v", parsed["trace_id"])
	}
}

func TestTraceHandler_WithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := telemetry.NewTraceHandler(baseHandler).
		WithAttrs([]slog.Attr{slog.String("static_attr", "foo")}).
		WithGroup("meta")

	logger := slog.New(handler)
	logger.Info("grouped message", "sub_key", "bar")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse json log: %v", err)
	}

	if got, want := parsed["static_attr"], "foo"; got != want {
		t.Errorf("static_attr got %v, want %v", got, want)
	}

	meta, ok := parsed["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta group object, got %v", parsed["meta"])
	}
	if got, want := meta["sub_key"], "bar"; got != want {
		t.Errorf("meta.sub_key got %v, want %v", got, want)
	}
}

func TestInitLogger_DefaultConfiguration(t *testing.T) {
	origLevel := os.Getenv("LOG_LEVEL")
	origFormat := os.Getenv("LOG_FORMAT")
	defer func() {
		_ = os.Setenv("LOG_LEVEL", origLevel)
		_ = os.Setenv("LOG_FORMAT", origFormat)
	}()

	_ = os.Setenv("LOG_LEVEL", "warn")
	_ = os.Setenv("LOG_FORMAT", "json")

	logger := telemetry.InitLogger("core-service")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Verify standard library log output redirect
	log.Println("redirected standard library log")
}

func TestInitLogger_DevDefaultsToDebug(t *testing.T) {
	origAppEnv := os.Getenv("APP_ENV")
	origLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("APP_ENV", origAppEnv)
		_ = os.Setenv("LOG_LEVEL", origLevel)
	}()

	_ = os.Setenv("APP_ENV", "development")
	_ = os.Unsetenv("LOG_LEVEL")

	logger := telemetry.InitLogger("dev-service")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// In development without LOG_LEVEL, debug messages should be enabled
	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug level to be enabled in development by default")
	}
}

func TestInitLogger_ProdDefaultsToInfo(t *testing.T) {
	origAppEnv := os.Getenv("APP_ENV")
	origLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		_ = os.Setenv("APP_ENV", origAppEnv)
		_ = os.Setenv("LOG_LEVEL", origLevel)
	}()

	_ = os.Setenv("APP_ENV", "production")
	_ = os.Unsetenv("LOG_LEVEL")

	logger := telemetry.InitLogger("prod-service")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// In production without LOG_LEVEL, info should be enabled, but debug should be disabled
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info level to be enabled in production by default")
	}
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug level to be disabled in production by default")
	}
}

func TestLoggerWithModule(t *testing.T) {
	logger := telemetry.LoggerWithModule("workout_execution")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}
