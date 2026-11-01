package middleware

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"time"

	"connectrpc.com/connect"
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
		return connect.NewError(connect.CodeInternal, err)
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
	return connect.NewError(code, errors.New(st.Message()))
}

// =========================================================================
// 1. Error Mapping Interceptor
// =========================================================================

type connectErrorMappingInterceptor struct{}

var _ connect.Interceptor = (*connectErrorMappingInterceptor)(nil)

// NewConnectErrorMappingInterceptor returns a Connect interceptor that translates
// standard gRPC status errors (from google.golang.org/grpc/status) into *connect.Error
// for both Unary and Streaming RPCs.
func NewConnectErrorMappingInterceptor() connect.Interceptor {
	return &connectErrorMappingInterceptor{}
}

func (i *connectErrorMappingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err == nil {
			return resp, nil
		}
		return resp, ToConnectError(err)
	}
}

func (i *connectErrorMappingInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err == nil {
			return nil
		}
		return ToConnectError(err)
	}
}

func (i *connectErrorMappingInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

// =========================================================================
// 2. Logging Interceptor
// =========================================================================

type connectLoggingInterceptor struct{}

var _ connect.Interceptor = (*connectLoggingInterceptor)(nil)

// NewConnectLoggingInterceptor returns a Connect interceptor that logs RPC duration,
// procedure, and status codes for both Unary and Streaming RPCs.
func NewConnectLoggingInterceptor() connect.Interceptor {
	return &connectLoggingInterceptor{}
}

func (i *connectLoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		duration := time.Since(start)

		procedure := "unknown"
		if req != nil {
			procedure = req.Spec().Procedure
		}
		if err != nil {
			connectErr := connect.NewError(connect.CodeInternal, err)
			var cErr *connect.Error
			if errors.As(err, &cErr) {
				connectErr = cErr
			}
			log.Printf("Connect Unary | Procedure: %s | Duration: %v | Code: %s | Message: %s",
				procedure, duration, connectErr.Code(), connectErr.Message())
		} else {
			log.Printf("Connect Unary | Procedure: %s | Duration: %v | Code: OK", procedure, duration)
		}
		return resp, err
	}
}

func (i *connectLoggingInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		procedure := conn.Spec().Procedure
		log.Printf("Connect Stream Start | Procedure: %s", procedure)

		err := next(ctx, conn)
		duration := time.Since(start)

		if err != nil {
			connectErr := connect.NewError(connect.CodeInternal, err)
			var cErr *connect.Error
			if errors.As(err, &cErr) {
				connectErr = cErr
			}
			log.Printf("Connect Stream End | Procedure: %s | Duration: %v | Code: %s | Message: %s",
				procedure, duration, connectErr.Code(), connectErr.Message())
		} else {
			log.Printf("Connect Stream End | Procedure: %s | Duration: %v | Code: OK", procedure, duration)
		}
		return err
	}
}

func (i *connectLoggingInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

// =========================================================================
// 3. Recovery Interceptor
// =========================================================================

type connectRecoveryInterceptor struct{}

var _ connect.Interceptor = (*connectRecoveryInterceptor)(nil)

// NewConnectRecoveryInterceptor returns a Connect interceptor that recovers from
// panics and returns an internal error for both Unary and Streaming RPCs.
func NewConnectRecoveryInterceptor() connect.Interceptor {
	return &connectRecoveryInterceptor{}
}

func (i *connectRecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in Connect Unary RPC handler: %v\nstacktrace: %s",
					r, string(debug.Stack()))
				err = connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
		}()
		return next(ctx, req)
	}
}

func (i *connectRecoveryInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in Connect Streaming RPC handler (%s): %v\nstacktrace: %s",
					conn.Spec().Procedure, r, string(debug.Stack()))
				err = connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
		}()
		return next(ctx, conn)
	}
}

func (i *connectRecoveryInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}
