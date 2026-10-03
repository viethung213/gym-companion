package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewConnectErrorMappingInterceptor returns a Connect interceptor that translates
// standard gRPC status errors (from google.golang.org/grpc/status) into *connect.Error
// with corresponding Connect codes and HTTP status mapping.
func NewConnectErrorMappingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			return resp, ToConnectError(err)
		}
	}
}

// ToConnectError maps standard gRPC status errors or raw errors into *connect.Error.
func ToConnectError(err error) error {
	if err == nil {
		return nil
	}
	if connectErr, ok := err.(*connect.Error); ok {
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

// NewConnectLoggingInterceptor returns a Connect interceptor that logs RPC duration, procedure, and status codes.
func NewConnectLoggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
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
}

// NewConnectRecoveryInterceptor returns a Connect interceptor that recovers from panics and returns an internal error.
func NewConnectRecoveryInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic recovered in Connect RPC handler: %v\nstacktrace: %s", r, string(debug.Stack()))
					err = connect.NewError(connect.CodeInternal, errors.New("internal server error"))
				}
			}()
			return next(ctx, req)
		}
	}
}

// NewConnectAuthInterceptor returns a Connect interceptor that validates JWT Bearer tokens.
func NewConnectAuthInterceptor(kp KeyProvider) connect.UnaryInterceptorFunc {
	keyCache := newRSAKeyCache(kp)

	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure
			required := isAuthRequired(procedure)

			tokenStr, err := extractTokenFromConnect(ctx, req)
			if err != nil {
				if errors.Is(err, errMissingMetadata) || errors.Is(err, errMissingAuthHeader) {
					if required {
						return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required: %w", err))
					}
					return next(ctx, req)
				}
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid authorization format: %w", err))
			}

			userID, role, err := parseAndValidateToken(ctx, tokenStr, keyCache)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
			}

			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)

			return next(ctx, req)
		}
	}
}

// NewConnectRateLimitInterceptor returns a Connect interceptor for rate limiting.
func NewConnectRateLimitInterceptor() connect.UnaryInterceptorFunc {
	registry := newRateLimiterRegistry()

	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure
			reqPerMin := getLimitForMethod(procedure)

			clientKey := extractPeerIPFromConnect(ctx, req) + ":" + procedure
			if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
				clientKey = userID + ":" + procedure
			}

			limiter := registry.GetLimiter(clientKey, reqPerMin)
			if !limiter.Allow() {
				return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limit exceeded: max %d requests per minute for %s", reqPerMin, procedure))
			}

			return next(ctx, req)
		}
	}
}

func extractTokenFromConnect(ctx context.Context, req connect.AnyRequest) (string, error) {
	authHeader := req.Header().Get("Authorization")
	if authHeader == "" {
		authHeader = req.Header().Get("authorization")
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

func extractPeerIPFromConnect(ctx context.Context, req connect.AnyRequest) string {
	if xff := req.Header().Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := req.Header().Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	peerIP := extractClientIP(ctx)
	if peerIP != "" {
		return peerIP
	}
	return "unknown"
}
