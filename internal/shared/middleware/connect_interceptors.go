package middleware

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/grpc/codes"
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
	case codes.OK:
		return nil
	case codes.Canceled:
		code = connect.CodeCanceled
	case codes.Unknown:
		code = connect.CodeUnknown
	case codes.InvalidArgument:
		code = connect.CodeInvalidArgument
	case codes.DeadlineExceeded:
		code = connect.CodeDeadlineExceeded
	case codes.NotFound:
		code = connect.CodeNotFound
	case codes.AlreadyExists:
		code = connect.CodeAlreadyExists
	case codes.PermissionDenied:
		code = connect.CodePermissionDenied
	case codes.ResourceExhausted:
		code = connect.CodeResourceExhausted
	case codes.FailedPrecondition:
		code = connect.CodeFailedPrecondition
	case codes.Aborted:
		code = connect.CodeAborted
	case codes.OutOfRange:
		code = connect.CodeOutOfRange
	case codes.Unimplemented:
		code = connect.CodeUnimplemented
	case codes.Internal:
		code = connect.CodeInternal
	case codes.Unavailable:
		code = connect.CodeUnavailable
	case codes.DataLoss:
		code = connect.CodeDataLoss
	case codes.Unauthenticated:
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
// procedure, and status codes for both Unary and Streaming RPCs.
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
					log.Printf("Connect RPC | Procedure: %s | Duration: %v | Code: %s | Message: %s",
						procedure, duration, cErr.Code(), cErr.Message())
				} else {
					log.Printf("Connect RPC | Procedure: %s | Duration: %v | Error: %v",
						procedure, duration, err)
				}
			} else {
				log.Printf("Connect RPC | Procedure: %s | Duration: %v | Code: OK", procedure, duration)
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
					log.Printf("panic recovered in Connect RPC handler (%s): %v\nstacktrace: %s",
						spec.Procedure, r, string(debug.Stack()))
					err = connect.NewError(connect.CodeInternal, "internal server error")
				}
			}()
			return next(ctx, spec, stream)
		}
	}
}
