package middleware

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

func TestConnectInternalAuthInterceptor(t *testing.T) {
	t.Parallel()

	secret := "test-internal-secret-auth-123"
	interceptor := NewConnectInternalAuthInterceptor(secret)

	procedure := "/test.Service/TestMethod"
	publicProcedure := authv1serviceconnect.AuthServiceGetJWKSProcedure

	t.Run("missing internal token is strictly rejected", func(t *testing.T) {
		server := connect.NewServer(interceptor)
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: procedure},
			Handler: func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
				return nil
			},
		})

		info := &connect.CallInfo{}
		err := server.Call(context.Background(), procedure, info, nil)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("public procedure bypasses internal token check", func(t *testing.T) {
		server := connect.NewServer(interceptor)
		called := false
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: publicProcedure},
			Handler: func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
				called = true
				return nil
			},
		})

		info := &connect.CallInfo{}
		err := server.Call(context.Background(), publicProcedure, info, nil)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("guest internal token succeeds and injects guest context", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("guest", "anonymous", "trace-public", time.Minute, secret)
		require.NoError(t, err)

		info := &connect.CallInfo{}
		info.RequestHeader().Set(HeaderInternalToken, tokenStr)
		info.RequestHeader().Set(HeaderRequestID, "trace-public")

		var gotUserID, gotRole string
		server := connect.NewServer(interceptor)
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: procedure},
			Handler: func(ctx context.Context, _ connect.Spec, _ connect.ServerStream) error {
				if u, ok := ctx.Value(UserIDKey).(string); ok {
					gotUserID = u
				}
				if r, ok := ctx.Value(UserRoleKey).(string); ok {
					gotRole = r
				}
				return nil
			},
		})

		err = server.Call(context.Background(), procedure, info, nil)
		require.NoError(t, err)
		assert.Equal(t, "guest", gotUserID)
		assert.Equal(t, "anonymous", gotRole)
	})

	t.Run("authenticated internal token succeeds and injects context", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("user-auth-1", "athlete", "trace-999", time.Minute, secret)
		require.NoError(t, err)

		info := &connect.CallInfo{}
		info.RequestHeader().Set(HeaderInternalToken, tokenStr)
		info.RequestHeader().Set(HeaderRequestID, "trace-999")

		var gotUserID, gotRole, gotReqID string
		server := connect.NewServer(interceptor)
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: procedure},
			Handler: func(ctx context.Context, _ connect.Spec, _ connect.ServerStream) error {
				if u, ok := ctx.Value(UserIDKey).(string); ok {
					gotUserID = u
				}
				if r, ok := ctx.Value(UserRoleKey).(string); ok {
					gotRole = r
				}
				if rid, ok := ctx.Value(RequestIDKey).(string); ok {
					gotReqID = rid
				}
				return nil
			},
		})

		err = server.Call(context.Background(), procedure, info, nil)
		require.NoError(t, err)
		assert.Equal(t, "user-auth-1", gotUserID)
		assert.Equal(t, "athlete", gotRole)
		assert.Equal(t, "trace-999", gotReqID)
	})

	t.Run("expired internal token returns CodeUnauthenticated", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("user-1", "user", "trace-expired", -1*time.Minute, secret)
		require.NoError(t, err)

		info := &connect.CallInfo{}
		info.RequestHeader().Set(HeaderInternalToken, tokenStr)

		server := connect.NewServer(interceptor)
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: procedure},
			Handler: func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
				return nil
			},
		})

		err = server.Call(context.Background(), procedure, info, nil)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("invalid token signature returns CodeUnauthenticated", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("user-1", "user", "trace-invalid", time.Minute, "wrong-secret")
		require.NoError(t, err)

		info := &connect.CallInfo{}
		info.RequestHeader().Set(HeaderInternalToken, tokenStr)

		server := connect.NewServer(interceptor)
		server.Register(connect.Method{
			Spec: connect.Spec{Procedure: procedure},
			Handler: func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
				return nil
			},
		})

		err = server.Call(context.Background(), procedure, info, nil)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}
