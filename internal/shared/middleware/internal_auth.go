package middleware

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

const (
	// HeaderInternalToken is the HTTP/gRPC metadata header for internal communication.
	HeaderInternalToken = "X-Internal-Token"
	// HeaderRequestID is the distributed tracing request identifier header.
	HeaderRequestID = "X-Request-ID"
	// RequestIDKey is the context key for tracing ID.
	RequestIDKey ContextKey = "requestID"
)

type connectInternalAuthInterceptor struct {
	secret string
}

// NewConnectInternalAuthInterceptor strictly validates X-Internal-Token on downstream microservices.
func NewConnectInternalAuthInterceptor(secret string) connect.Interceptor {
	return &connectInternalAuthInterceptor{secret: secret}
}

func (i *connectInternalAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		reqID := req.Header().Get(HeaderRequestID)
		if reqID != "" {
			ctx = context.WithValue(ctx, RequestIDKey, reqID)
		}

		// Public procedures defined in proto contracts (e.g. GetJWKS) do not require internal token
		if req.Spec().Procedure == authv1serviceconnect.AuthServiceGetJWKSProcedure {
			return next(ctx, req)
		}

		tokenStr := req.Header().Get(HeaderInternalToken)
		if tokenStr == "" {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("%w: missing internal token header", ErrUnauthorized),
			)
		}

		claims, err := internaltoken.Validate(tokenStr, i.secret)
		if err != nil {
			if errors.Is(err, internaltoken.ErrExpiredToken) {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("internal token expired: %w", err),
				)
			}
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid internal token: %w", err),
			)
		}

		// Inject verified user and role into context
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		if claims.RequestID != "" {
			ctx = context.WithValue(ctx, RequestIDKey, claims.RequestID)
		}

		return next(ctx, req)
	}
}

func (i *connectInternalAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *connectInternalAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		tokenStr := conn.RequestHeader().Get(HeaderInternalToken)
		reqID := conn.RequestHeader().Get(HeaderRequestID)
		if reqID != "" {
			ctx = context.WithValue(ctx, RequestIDKey, reqID)
		}

		if tokenStr == "" {
			return connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("%w: missing internal token header", ErrUnauthorized),
			)
		}

		claims, err := internaltoken.Validate(tokenStr, i.secret)
		if err != nil {
			if errors.Is(err, internaltoken.ErrExpiredToken) {
				return connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("internal token expired: %w", err),
				)
			}
			return connect.NewError(
				connect.CodeUnauthenticated,
				fmt.Errorf("invalid internal token: %w", err),
			)
		}

		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		if claims.RequestID != "" {
			ctx = context.WithValue(ctx, RequestIDKey, claims.RequestID)
		}

		return next(ctx, conn)
	}
}
