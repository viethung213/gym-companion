package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHTTPClient returns an *http.Client instrumented with OpenTelemetry tracing.
// Every outgoing HTTP request will automatically generate a Client span.
func NewHTTPClient(timeout time.Duration, base ...http.RoundTripper) *http.Client {
	transport := http.RoundTripper(http.DefaultTransport)
	if len(base) > 0 && base[0] != nil {
		transport = base[0]
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: otelhttp.NewTransport(transport),
	}
}

// WrapHTTPTransport wraps an existing http.RoundTripper with OpenTelemetry otelhttp.NewTransport.
func WrapHTTPTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}
