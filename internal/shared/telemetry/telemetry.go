// Package telemetry wires ADK's OpenTelemetry tracing to an OTLP collector
// (Jaeger in local development).
//
// ADK emits one generate_content span per LLM call carrying
// gen_ai.usage.input_tokens / output_tokens. That is what makes prompt-size
// regressions visible: an agent whose prompt silently accumulates another
// agent's event history shows up as an input_tokens count far larger than the
// payload it was handed. The span does not carry the prompt text itself.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	adktelemetry "google.golang.org/adk/v2/telemetry"
)

// TracesEndpointEnv is the standard OTLP/HTTP traces endpoint, including its
// path: "http://localhost:4318/v1/traces". Tracing stays off while it is
// unset, so nothing changes unless a developer opts in.
//
// Deliberately the traces-specific variable rather than the generic
// OTEL_EXPORTER_OTLP_ENDPOINT: the generic one also makes ADK stand up a log
// exporter, whose POSTs to /v1/logs a Jaeger all-in-one has nothing to answer.
const TracesEndpointEnv = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"

// ShutdownFunc flushes buffered spans and releases exporter resources. Spans
// are batched, so skipping it loses the tail of a run.
type ShutdownFunc func(context.Context) error

// Setup installs ADK's tracer provider as the global one and returns its
// shutdown hook plus whether tracing actually got enabled.
//
// The exporter itself is built by ADK from the OTEL_* environment; this only
// supplies the service name and performs the global registration.
func Setup(ctx context.Context, serviceName string) (ShutdownFunc, bool, error) {
	noop := func(context.Context) error { return nil }

	if strings.TrimSpace(os.Getenv(TracesEndpointEnv)) == "" {
		return noop, false, nil
	}

	// Schemaless: resource.Merge rejects two resources carrying different
	// schema URLs, and ADK merges this into its own schema-bearing default.
	res := resource.NewSchemaless(attribute.String("service.name", serviceName))

	providers, err := adktelemetry.New(ctx, adktelemetry.WithResource(res))
	if err != nil {
		return nil, false, fmt.Errorf("new adk telemetry providers: %w", err)
	}
	if providers.TracerProvider == nil {
		return noop, false, nil // endpoint set but ADK built no trace exporter
	}

	// ADK's internal tracer resolves through the global provider, so this
	// registration is what actually connects agent spans to the exporter.
	providers.SetGlobalOtelProviders()

	return providers.Shutdown, true, nil
}
