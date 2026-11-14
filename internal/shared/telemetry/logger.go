package telemetry

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const moduleContextKey contextKey = "telemetry_module"

type stdLogWriter struct {
	logger *slog.Logger
}

func (w *stdLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.logger.Info(msg)
	}
	return len(p), nil
}

// TraceHandler is an slog.Handler that enriches log records with OpenTelemetry
// trace_id and span_id when present in the context.Context.
type TraceHandler struct {
	slog.Handler
}

// NewTraceHandler wraps an existing slog.Handler with OpenTelemetry trace extraction.
func NewTraceHandler(h slog.Handler) *TraceHandler {
	return &TraceHandler{Handler: h}
}

// Handle appends trace_id, span_id, and module from context to the log record.
//
//nolint:gocritic // slog.Handler interface signature requires slog.Record by value
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			r.AddAttrs(
				slog.String("trace_id", span.SpanContext().TraceID().String()),
				slog.String("span_id", span.SpanContext().SpanID().String()),
			)
		}

		if moduleVal, ok := ctx.Value(moduleContextKey).(string); ok && moduleVal != "" {
			r.AddAttrs(slog.String("module", moduleVal))
		}
	}

	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new TraceHandler with the given attributes added.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new TraceHandler with the given group added.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

// InitLogger configures the global slog logger according to environment variables:
// - LOG_LEVEL: debug, info (default), warn, error
// - LOG_FORMAT: json (default for production/Loki), text (human-readable)
func InitLogger(serviceName string) *slog.Logger {
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	isDev := appEnv == "development" || appEnv == "local" || appEnv == "dev"

	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		if isDev {
			level = slog.LevelDebug
		} else {
			level = slog.LevelInfo
		}
	}

	formatStr := strings.ToLower(os.Getenv("LOG_FORMAT"))

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var baseHandler slog.Handler
	if formatStr == "text" || (formatStr == "" && isDev) {
		baseHandler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		baseHandler = slog.NewJSONHandler(os.Stdout, opts)
	}

	handler := NewTraceHandler(baseHandler).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Redirect standard library log.Printf/Println to slog so all legacy and background worker logs
	// are consistently formatted as structured JSON for Loki.
	log.SetFlags(0)
	log.SetOutput(&stdLogWriter{logger: logger})

	return logger
}

// ContextWithModule returns a new context containing the given module name.
func ContextWithModule(ctx context.Context, module string) context.Context {
	return context.WithValue(ctx, moduleContextKey, module)
}

// LoggerWithModule returns a sub-logger enriched with the module attribute.
func LoggerWithModule(module string) *slog.Logger {
	return slog.Default().With("module", module)
}
