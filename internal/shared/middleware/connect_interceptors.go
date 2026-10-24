package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
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

		procedure := req.Spec().Procedure
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

// =========================================================================
// 4. Authentication Interceptor
// =========================================================================

type connectAuthInterceptor struct {
	keyCache *rsaKeyCache
}

var _ connect.Interceptor = (*connectAuthInterceptor)(nil)

// NewConnectAuthInterceptor returns a Connect interceptor that validates JWT Bearer tokens
// and injects UserID and UserRole into context for both Unary and Streaming RPCs.
func NewConnectAuthInterceptor(kp KeyProvider) connect.Interceptor {
	return &connectAuthInterceptor{
		keyCache: newRSAKeyCache(kp),
	}
}

func (i *connectAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure
		required := isAuthRequired(procedure)

		tokenStr, err := extractTokenFromHeader(ctx, req.Header())
		if err != nil {
			if errors.Is(err, errMissingMetadata) || errors.Is(err, errMissingAuthHeader) {
				if required {
					return nil, connect.NewError(
						connect.CodeUnauthenticated,
						fmt.Errorf("authentication required: %w", err),
					)
				}
				return next(ctx, req)
			}
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid authorization format: %w", err),
			)
		}

		userID, role, err := parseAndValidateToken(ctx, tokenStr, i.keyCache)
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid token: %w", err),
			)
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, role)

		return next(ctx, req)
	}
}

func (i *connectAuthInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure
		required := isAuthRequired(procedure)

		tokenStr, err := extractTokenFromHeader(ctx, conn.RequestHeader())
		if err != nil {
			if errors.Is(err, errMissingMetadata) || errors.Is(err, errMissingAuthHeader) {
				if required {
					return connect.NewError(
						connect.CodeUnauthenticated,
						fmt.Errorf("authentication required: %w", err),
					)
				}
				return next(ctx, conn)
			}
			return connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid authorization format: %w", err),
			)
		}

		userID, role, err := parseAndValidateToken(ctx, tokenStr, i.keyCache)
		if err != nil {
			return connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid token: %w", err),
			)
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, role)

		return next(ctx, conn)
	}
}

func (i *connectAuthInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

// =========================================================================
// 5. Rate Limit Interceptor
// =========================================================================

type connectRateLimitInterceptor struct {
	registry *rateLimiterRegistry
}

var _ connect.Interceptor = (*connectRateLimitInterceptor)(nil)

// NewConnectRateLimitInterceptor returns a Connect interceptor for rate limiting
// both Unary and Streaming RPCs.
func NewConnectRateLimitInterceptor() connect.Interceptor {
	return &connectRateLimitInterceptor{
		registry: newRateLimiterRegistry(),
	}
}

func (i *connectRateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure
		reqPerMin := getLimitForMethod(procedure)

		clientKey := extractPeerIPFromHeader(ctx, req.Header()) + ":" + procedure
		if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
			clientKey = userID + ":" + procedure
		}

		limiter := i.registry.GetLimiter(clientKey, reqPerMin)
		if !limiter.Allow() {
			return nil, connect.NewError(
				connect.CodeResourceExhausted,
				fmt.Errorf("rate limit exceeded: max %d requests per minute for %s", reqPerMin, procedure),
			)
		}

		return next(ctx, req)
	}
}

func (i *connectRateLimitInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure
		reqPerMin := getLimitForMethod(procedure)

		clientKey := extractPeerIPFromHeader(ctx, conn.RequestHeader()) + ":" + procedure
		if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
			clientKey = userID + ":" + procedure
		}

		limiter := i.registry.GetLimiter(clientKey, reqPerMin)
		if !limiter.Allow() {
			return connect.NewError(
				connect.CodeResourceExhausted,
				fmt.Errorf("rate limit exceeded: max %d requests per minute for %s", reqPerMin, procedure),
			)
		}

		return next(ctx, conn)
	}
}

func (i *connectRateLimitInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

// =========================================================================
// Helpers
// =========================================================================

func extractTokenFromHeader(ctx context.Context, header http.Header) (string, error) {
	var authHeader string
	if header != nil {
		authHeader = header.Get("Authorization")
		if authHeader == "" {
			authHeader = header.Get("authorization")
		}
	}

	if authHeader == "" {
		// Fallback to gRPC metadata in context
		return extractToken(ctx)
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errInvalidAuthFormat
	}

	return parts[1], nil
}

func extractPeerIPFromHeader(ctx context.Context, header http.Header) string {
	if header != nil {
		if xff := header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	peerIP := extractClientIP(ctx)
	if peerIP != "" {
		return peerIP
	}
	return "unknown"
}
