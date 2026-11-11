package middleware

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect/v2"
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

// NewConnectInternalAuthInterceptor strictly validates X-Internal-Token on downstream microservices.
func NewConnectInternalAuthInterceptor(secret string) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			info, _ := connect.CallInfoForServerContext(ctx)
			var reqID, tokenStr string
			if info != nil {
				reqID = info.RequestHeader().Get(HeaderRequestID)
				tokenStr = info.RequestHeader().Get(HeaderInternalToken)
			}
			if reqID != "" {
				ctx = context.WithValue(ctx, RequestIDKey, reqID)
			}

			// Public procedures defined in proto contracts (e.g. GetJWKS) do not require internal token
			if spec.Procedure == authv1serviceconnect.AuthServiceGetJWKSProcedure {
				return next(ctx, spec, stream)
			}

			if tokenStr == "" {
				return connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("%w: missing internal token header", ErrUnauthorized).Error(),
				)
			}

			claims, err := internaltoken.Validate(tokenStr, secret)
			if err != nil {
				if errors.Is(err, internaltoken.ErrExpiredToken) {
					return connect.NewError(
						connect.CodeUnauthenticated,
						fmt.Errorf("internal token expired: %w", err).Error(),
					)
				}
				return connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("invalid internal token: %w", err).Error(),
				)
			}

			// Inject verified user and role into context
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
			if claims.RequestID != "" {
				ctx = context.WithValue(ctx, RequestIDKey, claims.RequestID)
			}

			return next(ctx, spec, stream)
		}
	}
}
