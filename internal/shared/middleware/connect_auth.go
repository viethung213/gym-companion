package middleware

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// ConnectAuthInterceptor returns a Connect interceptor that validates JWT Bearer tokens.
// It reuses the same RSA key cache and token validation logic as the gRPC interceptor.
func ConnectAuthInterceptor(kp KeyProvider) connect.UnaryInterceptorFunc {
	keyCache := newRSAKeyCache(kp)

	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			required := isAuthRequired(req.Spec().Procedure)

			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				if required {
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
				}
				return next(ctx, req)
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid authorization header format"))
			}

			userID, role, err := parseAndValidateToken(ctx, parts[1], keyCache)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			return next(ctx, req)
		}
	})
}
