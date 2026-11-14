package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToConnectError maps standard gRPC status errors or raw errors into *connect.Error.
func ToConnectError(err error) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr
	}
	st, ok := status.FromError(err)
	if !ok {
		return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	var code connect.Code
	switch st.Code() {
	case grpccodes.OK:
		return nil
	case grpccodes.Canceled:
		code = connect.CodeCanceled
	case grpccodes.Unknown:
		code = connect.CodeUnknown
	case grpccodes.InvalidArgument:
		code = connect.CodeInvalidArgument
	case grpccodes.DeadlineExceeded:
		code = connect.CodeDeadlineExceeded
	case grpccodes.NotFound:
		code = connect.CodeNotFound
	case grpccodes.AlreadyExists:
		code = connect.CodeAlreadyExists
	case grpccodes.PermissionDenied:
		code = connect.CodePermissionDenied
	case grpccodes.ResourceExhausted:
		code = connect.CodeResourceExhausted
	case grpccodes.FailedPrecondition:
		code = connect.CodeFailedPrecondition
	case grpccodes.Aborted:
		code = connect.CodeAborted
	case grpccodes.OutOfRange:
		code = connect.CodeOutOfRange
	case grpccodes.Unimplemented:
		code = connect.CodeUnimplemented
	case grpccodes.Internal:
		code = connect.CodeInternal
	case grpccodes.Unavailable:
		code = connect.CodeUnavailable
	case grpccodes.DataLoss:
		code = connect.CodeDataLoss
	case grpccodes.Unauthenticated:
		code = connect.CodeUnauthenticated
	default:
		code = connect.CodeUnknown
	}
	return connect.NewError(code, st.Message())
}

// =========================================================================
// 1. Error Mapping Interceptor
// =========================================================================

// NewConnectErrorMappingInterceptor returns a Connect server interceptor that translates
// standard gRPC status errors (from google.golang.org/grpc/status) into *connect.Error
// for both Unary and Streaming RPCs.
func NewConnectErrorMappingInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			err := next(ctx, spec, stream)
			if err == nil {
				return nil
			}
			return ToConnectError(err)
		}
	}
}

// =========================================================================
// 2. Logging Interceptor
// =========================================================================

// NewConnectLoggingInterceptor returns a Connect server interceptor that logs RPC duration,
// procedure, and status codes for both Unary and Streaming RPCs using structured slog.
func NewConnectLoggingInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			start := time.Now()
			procedure := spec.Procedure
			err := next(ctx, spec, stream)
			duration := time.Since(start)

			if err != nil {
				connectErr := ToConnectError(err)
				var cErr *connect.Error
				if errors.As(connectErr, &cErr) {
					slog.ErrorContext(ctx, "Connect RPC call failed",
						"procedure", procedure,
						"duration_ms", duration.Milliseconds(),
						"code", cErr.Code().String(),
						"error", cErr.Message(),
					)
				} else {
					slog.ErrorContext(ctx, "Connect RPC call failed",
						"procedure", procedure,
						"duration_ms", duration.Milliseconds(),
						"error", err.Error(),
					)
				}
			} else {
				slog.InfoContext(ctx, "Connect RPC call completed",
					"procedure", procedure,
					"duration_ms", duration.Milliseconds(),
					"code", "OK",
				)
			}
			return err
		}
	}
}

// =========================================================================
// 3. Recovery Interceptor
// =========================================================================

// NewConnectRecoveryInterceptor returns a Connect server interceptor that recovers from
// panics and returns an internal error for both Unary and Streaming RPCs.
func NewConnectRecoveryInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) (err error) {
			defer func() {
				if r := recover(); r != nil {
					slog.ErrorContext(ctx, "panic recovered in Connect RPC handler",
						"procedure", spec.Procedure,
						"panic", fmt.Sprint(r),
						"stack", string(debug.Stack()),
					)
					err = connect.NewError(connect.CodeInternal, "internal server error")
				}
			}()
			return next(ctx, spec, stream)
		}
	}
}

// =========================================================================
// 4. Tracing Interceptor
// =========================================================================

type connectHeaderCarrier struct {
	header *connect.Header
}

func (c connectHeaderCarrier) Get(key string) string {
	if c.header == nil {
		return ""
	}
	return c.header.Get(key)
}

func (c connectHeaderCarrier) Set(key, value string) {
	if c.header != nil {
		c.header.Set(key, value)
	}
}

func (c connectHeaderCarrier) Keys() []string {
	if c.header == nil {
		return nil
	}
	keys := make([]string, 0, c.header.Len())
	for k := range c.header.All() {
		keys = append(keys, k)
	}
	return keys
}

// extractModuleFromProcedure extracts the module name from a protobuf procedure path.
// Example: "/contracts.core.workout_execution.v1.service.WorkoutExecutionService/StartSession" -> "workout_execution"
func extractModuleFromProcedure(procedure string) string {
	parts := strings.Split(procedure, ".")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// NewConnectTracingInterceptor extracts distributed trace context from incoming Connect RPC
// requests, starts a Server span, and propagates traceparent into context and outgoing responses.
func NewConnectTracingInterceptor(serviceName ...string) connect.ServerInterceptor {
	tracerName := "fitai-backend"
	if len(serviceName) > 0 && serviceName[0] != "" {
		tracerName = serviceName[0]
	}
	tracer := otel.GetTracerProvider().Tracer(tracerName)

	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			// Extract W3C TraceContext from incoming headers if provided
			info, _ := connect.CallInfoForServerContext(ctx)
			if info != nil && info.RequestHeader() != nil {
				ctx = otel.GetTextMapPropagator().Extract(ctx, connectHeaderCarrier{header: info.RequestHeader()})
			}

			module := extractModuleFromProcedure(spec.Procedure)
			attrs := []attribute.KeyValue{
				attribute.String("rpc.system", "connect"),
				attribute.String("rpc.procedure", spec.Procedure),
			}
			if module != "" {
				attrs = append(attrs, attribute.String("app.module", module))
				ctx = telemetry.ContextWithModule(ctx, module)
			}

			ctx, span := tracer.Start(ctx, spec.Procedure,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attrs...),
			)
			defer span.End()

			err := next(ctx, spec, stream)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(otelcodes.Error, err.Error())
			} else {
				span.SetStatus(otelcodes.Ok, "OK")
			}
			return err
		}
	}
}
