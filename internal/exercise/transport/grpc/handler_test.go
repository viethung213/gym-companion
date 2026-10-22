//go:build unit

package grpc

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
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

func TestMeasurementTypeMappers(t *testing.T) {
	t.Parallel()

	t.Run("toProtoMeasurementType", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			give domain.MeasurementType
			want exercisemsg.ExerciseMeasurementType
		}{
			{
				give: domain.MeasurementTypeReps,
				want: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS,
			},
			{
				give: domain.MeasurementTypeDuration,
				want: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION,
			},
			{
				give: "UNKNOWN",
				want: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_UNSPECIFIED,
			},
		}

		for _, tt := range tests {
			if got := toProtoMeasurementType(tt.give); got != tt.want {
				t.Errorf("toProtoMeasurementType(%q) = %v, want %v", tt.give, got, tt.want)
			}
		}
	})

	t.Run("toDomainMeasurementType", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			give exercisemsg.ExerciseMeasurementType
			want domain.MeasurementType
		}{
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS,
				want: domain.MeasurementTypeReps,
			},
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION,
				want: domain.MeasurementTypeDuration,
			},
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_UNSPECIFIED,
				want: "",
			},
		}

		for _, tt := range tests {
			if got := toDomainMeasurementType(tt.give); got != tt.want {
				t.Errorf("toDomainMeasurementType(%v) = %q, want %q", tt.give, got, tt.want)
			}
		}
	})

	t.Run("toDomainMeasurementTypeString", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			give exercisemsg.ExerciseMeasurementType
			want string
		}{
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS,
				want: "REPS",
			},
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION,
				want: "DURATION",
			},
			{
				give: exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_UNSPECIFIED,
				want: "",
			},
		}

		for _, tt := range tests {
			if got := toDomainMeasurementTypeString(tt.give); got != tt.want {
				t.Errorf("toDomainMeasurementTypeString(%v) = %q, want %q", tt.give, got, tt.want)
			}
		}
	})

	t.Run("toProtoExercise", func(t *testing.T) {
		t.Parallel()
		info := &domain.Info{
			ID:              "ex-1",
			Name:            "Plank",
			MeasurementType: domain.MeasurementTypeDuration,
		}
		protoEx := toProtoExercise(info)
		if got, want := protoEx.GetMeasurementType(), exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
