package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

func TestConnectStreamingInternalAuthInterceptor(t *testing.T) {
	t.Parallel()

	secret := "test-internal-secret-streaming-123"
	interceptor := NewConnectInternalAuthInterceptor(secret)

	t.Run("missing internal token is strictly rejected", func(t *testing.T) {
		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{
				Procedure:  "/contracts.exercise.v1.ExerciseService/GetMotionSpecification",
				StreamType: connect.StreamTypeServer,
			},
			reqHeader: make(http.Header),
		}

		handler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
			return nil
		})

		err := handler(context.Background(), conn)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("guest internal token succeeds and injects guest context", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("guest", "anonymous", "trace-public", time.Minute, secret)
		require.NoError(t, err)

		header := make(http.Header)
		header.Set(HeaderInternalToken, tokenStr)
		header.Set(HeaderRequestID, "trace-public")

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{
				Procedure:  "/contracts.exercise.v1.ExerciseService/GetMotionSpecification",
				StreamType: connect.StreamTypeServer,
			},
			reqHeader: header,
		}

		var gotUserID, gotRole string
		handler := interceptor.WrapStreamingHandler(func(ctx context.Context, _ connect.StreamingHandlerConn) error {
			if u, ok := ctx.Value(UserIDKey).(string); ok {
				gotUserID = u
			}
			if r, ok := ctx.Value(UserRoleKey).(string); ok {
				gotRole = r
			}
			return nil
		})

		err = handler(context.Background(), conn)
		require.NoError(t, err)
		assert.Equal(t, "guest", gotUserID)
		assert.Equal(t, "anonymous", gotRole)
	})

	t.Run("authenticated internal token succeeds and injects context", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("user-stream-1", "athlete", "trace-999", time.Minute, secret)
		require.NoError(t, err)

		header := make(http.Header)
		header.Set(HeaderInternalToken, tokenStr)
		header.Set(HeaderRequestID, "trace-999")

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{
				Procedure:  "/contracts.core.solo_match.v1.service.SoloMatchService/JoinQueue",
				StreamType: connect.StreamTypeServer,
			},
			reqHeader: header,
		}

		var gotUserID, gotRole, gotReqID string
		handler := interceptor.WrapStreamingHandler(func(ctx context.Context, _ connect.StreamingHandlerConn) error {
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
		})

		err = handler(context.Background(), conn)
		require.NoError(t, err)
		assert.Equal(t, "user-stream-1", gotUserID)
		assert.Equal(t, "athlete", gotRole)
		assert.Equal(t, "trace-999", gotReqID)
	})

	t.Run("expired internal token is rejected", func(t *testing.T) {
		tokenStr, err := internaltoken.Generate("user-stream-1", "athlete", "trace-999", 5*time.Millisecond, secret)
		require.NoError(t, err)

		time.Sleep(15 * time.Millisecond)

		header := make(http.Header)
		header.Set(HeaderInternalToken, tokenStr)

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{
				Procedure:  "/contracts.core.solo_match.v1.service.SoloMatchService/JoinQueue",
				StreamType: connect.StreamTypeServer,
			},
			reqHeader: header,
		}

		handler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
			return nil
		})

		err = handler(context.Background(), conn)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}
