//go:build unit

package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time {
	return c.now
}

type mockIDs struct {
	id string
}

func (i *mockIDs) NewID() (string, error) {
	return i.id, nil
}

func TestNewExerciseServer(t *testing.T) {
	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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

type mockRepoForHandler struct {
	exercise *domain.Exercise
	err      error
}

func (m *mockRepoForHandler) Save(_ context.Context, _ *domain.Exercise, _ *domain.Event) error {
	return nil
}

func (m *mockRepoForHandler) FindByID(_ context.Context, _ string) (*domain.Exercise, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.exercise != nil {
		return m.exercise, nil
	}
	return nil, domain.ErrExerciseNotFound
}

func (m *mockRepoForHandler) SearchActive(_ context.Context, _ *port.SearchFilters) ([]*domain.Exercise, int32, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.exercise != nil {
		return []*domain.Exercise{m.exercise}, 1, nil
	}
	return nil, 0, nil
}

func (m *mockRepoForHandler) GetRandomAISupported(_ context.Context) (*domain.Exercise, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.exercise, nil
}

func (m *mockRepoForHandler) GetMetadata(_ context.Context) (port.Metadata, error) {
	return port.Metadata{}, nil
}

func (m *mockRepoForHandler) CreateBodyPart(_ context.Context, _ *port.BodyPart) error {
	return nil
}

func (m *mockRepoForHandler) GetBodyPart(_ context.Context, _ string) (*port.BodyPart, error) {
	return nil, nil
}

func (m *mockRepoForHandler) ListBodyParts(_ context.Context, _, _ int) ([]port.BodyPart, int, error) {
	return nil, 0, nil
}

func (m *mockRepoForHandler) UpdateBodyPart(_ context.Context, _ *port.BodyPart) error {
	return nil
}

func (m *mockRepoForHandler) DeleteBodyPart(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoForHandler) CreateEquipment(_ context.Context, _ *port.Equipment) error {
	return nil
}

func (m *mockRepoForHandler) GetEquipment(_ context.Context, _ string) (*port.Equipment, error) {
	return nil, nil
}

func (m *mockRepoForHandler) ListEquipments(_ context.Context, _, _ int) ([]port.Equipment, int, error) {
	return nil, 0, nil
}

func (m *mockRepoForHandler) UpdateEquipment(_ context.Context, _ *port.Equipment) error {
	return nil
}

func (m *mockRepoForHandler) DeleteEquipment(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoForHandler) CreateMuscle(_ context.Context, _ *port.Muscle) error {
	return nil
}

func (m *mockRepoForHandler) GetMuscle(_ context.Context, _ string) (*port.Muscle, error) {
	return nil, nil
}

func (m *mockRepoForHandler) ListMuscles(_ context.Context, _ string, _, _ int) ([]port.Muscle, int, error) {
	return nil, 0, nil
}

func (m *mockRepoForHandler) UpdateMuscle(_ context.Context, _ *port.Muscle) error {
	return nil
}

func (m *mockRepoForHandler) DeleteMuscle(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoForHandler) CreateTag(_ context.Context, _ *port.Tag) error {
	return nil
}

func (m *mockRepoForHandler) GetTag(_ context.Context, _ string) (*port.Tag, error) {
	return nil, nil
}

func (m *mockRepoForHandler) ListTags(_ context.Context, _, _ int) ([]port.Tag, int, error) {
	return nil, 0, nil
}

func (m *mockRepoForHandler) UpdateTag(_ context.Context, _ *port.Tag) error {
	return nil
}

func (m *mockRepoForHandler) DeleteTag(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoForHandler) SetAISupportedWithOutboxLog(_ context.Context, _ string, _ bool, _ *port.OutboxLogRecord) error {
	return nil
}

func TestExerciseServer_GetRandomExercise(t *testing.T) {
	now := time.Now().UTC()
	ex, err := domain.RehydrateExercise(domain.Info{
		ID:              "ex-123",
		Name:            "Push-up",
		Instructions:    "Push up description",
		BodyPartID:      "chest",
		EquipmentID:     "none",
		TargetMuscleID:  "pectoralis",
		Difficulty:      "BEGINNER",
		Status:          domain.StatusActive,
		HasAISupported:  true,
		MeasurementType: domain.MeasurementTypeReps,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("unexpected rehydrate error: %v", err)
	}

	repo := &mockRepoForHandler{exercise: ex}
	getRandomHdlr := query.NewGetRandomExerciseHandler(repo)
	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, getRandomHdlr, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)

	ctx := context.Background()
	req := &exercisemsg.GetRandomExerciseRequest{}

	resp, err := srv.GetRandomExercise(ctx, req)
	if err != nil {
		t.Fatalf("GetRandomExercise failed: %v", err)
	}
	if resp.GetExercise().GetId() != "ex-123" {
		t.Errorf("got id %q, want %q", resp.GetExercise().GetId(), "ex-123")
	}

	// Test Connect Handler wrapper
	connectHdlr := NewConnectExerciseHandler(srv)
	connectResp, err := connectHdlr.GetRandomExercise(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("ConnectExerciseHandler.GetRandomExercise failed: %v", err)
	}
	if connectResp.Msg.GetExercise().GetId() != "ex-123" {
		t.Errorf("got connect id %q, want %q", connectResp.Msg.GetExercise().GetId(), "ex-123")
	}

	// Test Error case
	repoErr := &mockRepoForHandler{err: domain.ErrExerciseNotFound}
	srvErr := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, query.NewGetRandomExerciseHandler(repoErr), nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)
	_, err = srvErr.GetRandomExercise(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
		t.Errorf("got status %v, want NotFound", err)
	}
}

func TestSearchExercises_TotalCountAndStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	now := time.Now().UTC()
	ex, err := domain.NewExercise(domain.Info{
		ID:              "ex-search-1",
		Name:            "Barbell Bench Press",
		BodyPartID:      "bp-chest",
		EquipmentID:     "eq-barbell",
		TargetMuscleID:  "m-pectoralis",
		Instructions:    "Press the bar up",
		Difficulty:      "INTERMEDIATE",
		Status:          domain.StatusActive,
		MeasurementType: domain.MeasurementTypeReps,
	}, now)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	repo := &mockRepoForHandler{exercise: ex}
	searchHdlr := query.NewSearchExercisesHandler(repo)
	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, nil, nil, nil, searchHdlr,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)

	req := &exercisemsg.SearchExercisesRequest{
		Keyword:            "Bench",
		Status:             exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE,
		IncludeAllStatuses: false,
		Limit:              10,
		Offset:             0,
	}

	resp, err := srv.SearchExercises(ctx, req)
	if err != nil {
		t.Fatalf("SearchExercises failed: %v", err)
	}

	if resp.GetTotalCount() != 1 {
		t.Errorf("got TotalCount %d, want 1", resp.GetTotalCount())
	}
	if len(resp.GetExercises()) != 1 {
		t.Fatalf("got %d exercises, want 1", len(resp.GetExercises()))
	}
	if resp.GetExercises()[0].GetId() != "ex-search-1" {
		t.Errorf("got id %q, want ex-search-1", resp.GetExercises()[0].GetId())
	}

	// ConnectRPC wrapper
	connectHdlr := NewConnectExerciseHandler(srv)
	cResp, err := connectHdlr.SearchExercises(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("ConnectExerciseHandler.SearchExercises failed: %v", err)
	}
	if cResp.Msg.GetTotalCount() != 1 {
		t.Errorf("got connect TotalCount %d, want 1", cResp.Msg.GetTotalCount())
	}
}

func TestRestoreExercise_RPC(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()

	ex, err := domain.NewExercise(domain.Info{
		ID:              "ex-restore-test",
		Name:            "Barbell Squat",
		BodyPartID:      "bp-legs",
		EquipmentID:     "eq-barbell",
		TargetMuscleID:  "m-quads",
		Instructions:    "Squat down and stand up",
		Difficulty:      "INTERMEDIATE",
		Status:          domain.StatusArchived,
		MeasurementType: domain.MeasurementTypeReps,
	}, now)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	repo := &mockRepoForHandler{exercise: ex}
	clock := &mockClock{now: now}
	ids := &mockIDs{id: "evt-restore-1"}
	restoreHdlr := command.NewRestoreExerciseHandler(repo, clock, ids)

	srv := NewExerciseServer(
		nil, nil, nil, nil, nil, restoreHdlr, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)

	// Admin context
	adminCtx := context.WithValue(context.Background(), middleware.UserRoleKey, "Admin")
	adminCtx = context.WithValue(adminCtx, middleware.UserIDKey, "admin-123")

	req := &exercisemsg.RestoreExerciseRequest{Id: "ex-restore-test"}
	resp, err := srv.RestoreExercise(adminCtx, req)
	if err != nil {
		t.Fatalf("RestoreExercise failed: %v", err)
	}

	if resp.GetExercise().GetStatus() != exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE {
		t.Errorf("got status %v, want ACTIVE", resp.GetExercise().GetStatus())
	}

	// Re-archive exercise to test ConnectRPC restore
	if err := ex.Archive(now.Add(3 * time.Minute)); err != nil {
		t.Fatalf("re-archive failed: %v", err)
	}

	// ConnectRPC wrapper
	connectHdlr := NewConnectExerciseHandler(srv)
	cResp, err := connectHdlr.RestoreExercise(adminCtx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("Connect.RestoreExercise failed: %v", err)
	}
	if cResp.Msg.GetExercise().GetStatus() != exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE {
		t.Errorf("got connect status %v, want ACTIVE", cResp.Msg.GetExercise().GetStatus())
	}
}
