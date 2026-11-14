package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer initializes an OpenTelemetry TracerProvider with an OTLP exporter.
// By default, it uses high-performance gRPC (port 4317) for internal inter-service
// and agent communication. If endpoint ends with :4318 or OTEL protocol specifies HTTP,
// it gracefully falls back to OTLP HTTP exporter.
// It returns a shutdown function to be invoked on application termination.
// If OTEL_TRACES_ENABLED is set to false, a no-op shutdown is returned.
func InitTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Tracing is enabled by default in all environments unless explicitly set to false.
	if strings.EqualFold(os.Getenv("OTEL_TRACES_ENABLED"), "false") {
		slog.Info("OpenTelemetry tracing is explicitly disabled", "service", serviceName)
		return func(context.Context) error { return nil }, nil
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}

	// Default fallback: automatically route to Alloy gRPC receiver (port 4317).
	// In production docker network, Alloy service is named 'alloy'. In local dev, it is 'localhost'.
	if endpoint == "" {
		appEnv := strings.ToLower(os.Getenv("APP_ENV"))
		if appEnv == "production" {
			endpoint = "alloy:4317"
		} else {
			endpoint = "localhost:4317"
		}
	}

	// Strip URL scheme if present (e.g. http://localhost:4317 -> localhost:4317)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	// Determine insecure mode
	insecure := true
	if val := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); val != "" {
		insecure = strings.EqualFold(val, "true")
	} else if strings.HasSuffix(endpoint, ":443") {
		insecure = false
	}

	exporter, protocol, err := createExporter(ctx, endpoint, insecure)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	hostname, _ := os.Hostname()
	attrs := []attribute.KeyValue{
		attribute.String("service.name", serviceName),
		attribute.String("host.name", hostname),
	}

	if env := os.Getenv("APP_ENV"); env != "" {
		attrs = append(attrs, attribute.String("deployment.environment", env))
	}
	if tag := os.Getenv("IMAGE_TAG"); tag != "" {
		attrs = append(attrs, attribute.String("service.version", tag))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
	)
	if err != nil {
		slog.Warn("Failed to create full OpenTelemetry resource, falling back to minimal", "error", err)
		res = resource.NewSchemaless(attrs...)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("OpenTelemetry tracer provider initialized",
		"service", serviceName,
		"endpoint", endpoint,
		"protocol", protocol,
	)

	return tp.Shutdown, nil
}

func createExporter(ctx context.Context, endpoint string, insecure bool) (sdktrace.SpanExporter, string, error) {
	protocol := strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))

	// Graceful fallback to HTTP if explicitly configured or port :4318 is used
	if protocol == "http/protobuf" || protocol == "http/json" || strings.HasSuffix(endpoint, ":4318") {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}
		if insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exp, err := otlptracehttp.New(ctx, opts...)
		return exp, "http", err
	}

	// Default internal standard: gRPC over port 4317 (HTTP/2 binary multiplexing, minimal overhead)
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exp, err := otlptracegrpc.New(ctx, opts...)
	return exp, "grpc", err
}
