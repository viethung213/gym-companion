//go:build unit

package grpc

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewExerciseServer(t *testing.T) {
	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)
	if srv == nil {
		t.Fatal("expected non-nil ExerciseServer")
	}
}

func TestNewConnectExerciseHandler(t *testing.T) {
	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)
	connectHdlr := NewConnectExerciseHandler(srv)
	if connectHdlr == nil {
		t.Fatal("expected non-nil ConnectExerciseHandler")
	}
}

func TestRpcErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "unauthorized error",
			err:      middleware.ErrUnauthorized,
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "forbidden error",
			err:      middleware.ErrForbidden,
			wantCode: codes.PermissionDenied,
		},
		{
			name:     "exercise not found",
			err:      domain.ErrExerciseNotFound,
			wantCode: codes.NotFound,
		},
		{
			name:     "invalid exercise",
			err:      domain.ErrInvalidExercise,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "internal error fallback",
			err:      errors.New("unknown DB failure"),
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := rpcError(tt.err)
			st, ok := status.FromError(gotErr)
			if !ok {
				t.Fatalf("rpcError(%v) did not return gRPC status error", tt.err)
			}
			if st.Code() != tt.wantCode {
				t.Errorf("rpcError(%v) code = %v, want %v", tt.err, st.Code(), tt.wantCode)
			}
		})
	}
}
