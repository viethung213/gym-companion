package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing ensures every request has an X-Request-ID and propagates W3C TraceContext.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract existing W3C trace context if present from upstream/client
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		tracer := otel.GetTracerProvider().Tracer("fitai-gateway")
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			if span.SpanContext().IsValid() {
				reqID = span.SpanContext().TraceID().String()
			} else {
				reqID = uuid.New().String()
			}
		}

		w.Header().Set(HeaderRequestID, reqID)
		r.Header.Set(HeaderRequestID, reqID)

		// Inject traceparent to outgoing headers for downstream microservices
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
