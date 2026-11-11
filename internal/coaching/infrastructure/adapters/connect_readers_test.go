package adapters_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/adapters"
	workoutexecutionmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service/workoutexecutionv1serviceconnect"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service/exercisev1serviceconnect"
	profilemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service/profilev1serviceconnect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockProfileHandler implements profilev1serviceconnect.ProfileServiceHandler
type mockProfileHandler struct {
	profilev1serviceconnect.UnimplementedProfileServiceHandler
	getProfileFn func(ctx context.Context, req *profilemsg.GetProfileRequest) (*profilemsg.GetProfileResponse, error)
}

func (m *mockProfileHandler) GetProfile(ctx context.Context, req *profilemsg.GetProfileRequest) (*profilemsg.GetProfileResponse, error) {
	if m.getProfileFn != nil {
		return m.getProfileFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, "unimplemented")
}

// mockExerciseHandler implements exercisev1serviceconnect.ExerciseServiceHandler
type mockExerciseHandler struct {
	exercisev1serviceconnect.UnimplementedExerciseServiceHandler
	searchFn  func(ctx context.Context, req *exercisemsg.SearchExercisesRequest) (*exercisemsg.SearchExercisesResponse, error)
	getByIDFn func(ctx context.Context, req *exercisemsg.GetExerciseRequest) (*exercisemsg.GetExerciseResponse, error)
}

func (m *mockExerciseHandler) SearchExercises(ctx context.Context, req *exercisemsg.SearchExercisesRequest) (*exercisemsg.SearchExercisesResponse, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, "unimplemented")
}

func (m *mockExerciseHandler) GetExercise(ctx context.Context, req *exercisemsg.GetExerciseRequest) (*exercisemsg.GetExerciseResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, "unimplemented")
}

// mockWorkoutHandler implements workoutexecutionv1serviceconnect.AdminWorkoutExecutionServiceHandler
type mockWorkoutHandler struct {
	workoutexecutionv1serviceconnect.UnimplementedAdminWorkoutExecutionServiceHandler
	historyFn func(ctx context.Context, req *workoutexecutionmsg.AdminGetWorkoutHistoryRequest) (*workoutexecutionmsg.AdminGetWorkoutHistoryResponse, error)
	prFn      func(ctx context.Context, req *workoutexecutionmsg.AdminGetPersonalRecordsRequest) (*workoutexecutionmsg.AdminGetPersonalRecordsResponse, error)
}

func (m *mockWorkoutHandler) AdminGetWorkoutHistory(ctx context.Context, req *workoutexecutionmsg.AdminGetWorkoutHistoryRequest) (*workoutexecutionmsg.AdminGetWorkoutHistoryResponse, error) {
	if m.historyFn != nil {
		return m.historyFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, "unimplemented")
}

func (m *mockWorkoutHandler) AdminGetPersonalRecords(ctx context.Context, req *workoutexecutionmsg.AdminGetPersonalRecordsRequest) (*workoutexecutionmsg.AdminGetPersonalRecordsResponse, error) {
	if m.prFn != nil {
		return m.prFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, "unimplemented")
}

func TestConnectUserProfileReader_Success(t *testing.T) {
	mockHandler := &mockProfileHandler{
		getProfileFn: func(ctx context.Context, req *profilemsg.GetProfileRequest) (*profilemsg.GetProfileResponse, error) {
			assert.Equal(t, "user-123", req.GetUserId())
			if info, ok := connect.CallInfoForServerContext(ctx); ok {
				assert.NotEmpty(t, info.RequestHeader().Get("X-Internal-Token"))
			}

			return &profilemsg.GetProfileResponse{
				UserId:                "user-123",
				WeightKg:              80.5,
				HeightCm:              178.0,
				Age:                   28,
				Goals:                 []string{"hypertrophy", "strength"},
				AvailableEquipment:    []string{"barbell", "dumbbell"},
				PreferredMuscleGroups: []string{"chest", "triceps"},
				PreferredWorkoutTimes: []string{
					`{"mon": ["06:00-07:30"]}`,
					`[{"day_of_week":3, "start_time":"18:00", "end_time":"19:30"}]`,
					"fri: 07:00-08:30",
				},
				Injuries: []*profilemsg.Injury{
					{
						MuscleGroup: "shoulder",
						ReportedAt:  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
						IsRecovered: false,
						Notes:       "Mild impingement",
					},
					{
						MuscleGroup: "knee",
						ReportedAt:  time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
						IsRecovered: true,
						RecoveredAt: time.Now().Format(time.RFC3339),
					},
				},
				UpdatedAt: time.Now().Format(time.RFC3339),
			}, nil
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	profilev1serviceconnect.RegisterProfileServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectUserProfileReader(server.URL, "test-secret", server.Client())
	profile, err := reader.GetProfile(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", profile.UserID)
	assert.Equal(t, 80.5, profile.WeightKg)
	assert.Equal(t, 178.0, profile.HeightCm)
	assert.Equal(t, 28, profile.Age)
	assert.Equal(t, "hypertrophy", profile.PrimaryGoal)
	assert.Equal(t, []string{"barbell", "dumbbell"}, profile.AvailableEquipment)
	assert.Equal(t, []string{"chest", "triceps"}, profile.PreferredMuscleGroups)
	assert.Len(t, profile.AvailableSlots, 3)
	assert.Len(t, profile.ActiveInjuries, 1)
	assert.Equal(t, "shoulder", profile.ActiveInjuries[0].MuscleGroup)
}

func TestConnectUserProfileReader_NotFound_Error(t *testing.T) {
	mockHandler := &mockProfileHandler{
		getProfileFn: func(_ context.Context, _ *profilemsg.GetProfileRequest) (*profilemsg.GetProfileResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, "profile not found")
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	profilev1serviceconnect.RegisterProfileServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectUserProfileReader(server.URL, "test-secret", server.Client())
	_, err := reader.GetProfile(context.Background(), "unknown-user")

	require.Error(t, err)
}

func TestConnectUserProfileReader_EmptyUserID(t *testing.T) {
	reader := adapters.NewConnectUserProfileReader("http://localhost:8000", "test-secret", nil)
	_, err := reader.GetProfile(context.Background(), "")
	assert.Error(t, err)
}

func TestConnectExerciseCatalogReader_SearchByFilter(t *testing.T) {
	mockHandler := &mockExerciseHandler{
		searchFn: func(_ context.Context, req *exercisemsg.SearchExercisesRequest) (*exercisemsg.SearchExercisesResponse, error) {
			assert.Equal(t, "chest", req.GetTargetMuscleId())
			assert.Equal(t, exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE, req.GetStatus())

			return &exercisemsg.SearchExercisesResponse{
				Exercises: []*exercisemsg.ExerciseInfo{
					{
						Id:             "bench-press",
						Name:           "Bench Press",
						TargetMuscleId: "chest",
						EquipmentId:    "barbell",
						Difficulty:     "INTERMEDIATE",
					},
					{
						Id:             "cable-crossover",
						Name:           "Cable Crossover",
						TargetMuscleId: "chest",
						EquipmentId:    "cable",
						Difficulty:     "BEGINNER",
					},
					{
						Id:             "push-up",
						Name:           "Push Up",
						TargetMuscleId: "chest",
						EquipmentId:    "bodyweight",
						Difficulty:     "BEGINNER",
					},
				},
				TotalCount: 3,
			}, nil
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	exercisev1serviceconnect.RegisterExerciseServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectExerciseCatalogReader(server.URL, "test-secret", server.Client())
	exercises, err := reader.SearchByFilter(context.Background(), &port.ExerciseFilter{
		TargetMuscleID: "chest",
	})

	require.NoError(t, err)
	require.Len(t, exercises, 3)

	assert.Equal(t, "bench-press", exercises[0].ExerciseID)
	assert.False(t, exercises[0].IsBodyweight)
	assert.False(t, exercises[0].IsMachineOrCable)

	assert.Equal(t, "cable-crossover", exercises[1].ExerciseID)
	assert.True(t, exercises[1].IsMachineOrCable)

	assert.Equal(t, "push-up", exercises[2].ExerciseID)
	assert.True(t, exercises[2].IsBodyweight)
}

func TestConnectExerciseCatalogReader_GetByID_SuccessAndNotFound(t *testing.T) {
	mockHandler := &mockExerciseHandler{
		getByIDFn: func(_ context.Context, req *exercisemsg.GetExerciseRequest) (*exercisemsg.GetExerciseResponse, error) {
			if req.GetId() == "bench-press" {
				return &exercisemsg.GetExerciseResponse{
					Exercise: &exercisemsg.ExerciseInfo{
						Id:             "bench-press",
						Name:           "Bench Press",
						TargetMuscleId: "chest",
						EquipmentId:    "barbell",
					},
				}, nil
			}
			return nil, connect.NewError(connect.CodeNotFound, "exercise not found")
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	exercisev1serviceconnect.RegisterExerciseServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectExerciseCatalogReader(server.URL, "test-secret", server.Client())

	// Success
	ex, err := reader.GetByID(context.Background(), "bench-press")
	require.NoError(t, err)
	assert.Equal(t, "bench-press", ex.ExerciseID)

	// Not Found
	_, errNotFound := reader.GetByID(context.Background(), "non-existent")
	assert.ErrorIs(t, errNotFound, port.ErrExerciseNotFound)

	// Empty ID
	_, errEmpty := reader.GetByID(context.Background(), "")
	assert.ErrorIs(t, errEmpty, port.ErrExerciseNotFound)
}

func TestConnectWorkoutSessionReader_GetRecentSessions(t *testing.T) {
	now := time.Now()
	formScore := float32(88.5)

	mockHandler := &mockWorkoutHandler{
		historyFn: func(_ context.Context, req *workoutexecutionmsg.AdminGetWorkoutHistoryRequest) (*workoutexecutionmsg.AdminGetWorkoutHistoryResponse, error) {
			assert.Equal(t, "user-456", req.GetUserId())

			return &workoutexecutionmsg.AdminGetWorkoutHistoryResponse{
				Sessions: []*workoutexecutionmsg.WorkoutSessionSummary{
					{
						SessionId:        "sess-1",
						Date:             timestamppb.New(now.Add(-2 * time.Hour)),
						TotalSets:        5,
						AverageFormScore: &formScore,
					},
					{
						SessionId: "sess-2",
						Date:      timestamppb.New(now.Add(-10 * 24 * time.Hour)),
						TotalSets: 4,
					},
				},
			}, nil
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	workoutexecutionv1serviceconnect.RegisterAdminWorkoutExecutionServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectWorkoutSessionReader(server.URL, "test-secret", server.Client())

	// Filter since 3 days ago -> should only return sess-1
	sessions, err := reader.GetRecentSessions(context.Background(), "user-456", now.Add(-3*24*time.Hour))
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "sess-1", sessions[0].SessionID)
	assert.Equal(t, 5, sessions[0].TotalSets)
	assert.InDelta(t, 88.5, sessions[0].AverageFormScore, 0.01)

	// Empty user ID
	_, errEmpty := reader.GetRecentSessions(context.Background(), "", time.Time{})
	assert.Error(t, errEmpty)
}

func TestConnectWorkoutSessionReader_GetSetLogs(t *testing.T) {
	now := time.Now()

	mockHandler := &mockWorkoutHandler{
		prFn: func(_ context.Context, req *workoutexecutionmsg.AdminGetPersonalRecordsRequest) (*workoutexecutionmsg.AdminGetPersonalRecordsResponse, error) {
			assert.Equal(t, "user-456", req.GetUserId())
			if len(req.GetExerciseIds()) > 0 && req.GetExerciseIds()[0] == "bench-press" {
				return &workoutexecutionmsg.AdminGetPersonalRecordsResponse{
					Records: []*workoutexecutionmsg.PersonalRecord{
						{
							ExerciseId: "bench-press",
							Weight:     100.0,
							Reps:       5,
							AchievedAt: timestamppb.New(now),
						},
					},
				}, nil
			}
			return &workoutexecutionmsg.AdminGetPersonalRecordsResponse{
				Records: []*workoutexecutionmsg.PersonalRecord{},
			}, nil
		},
	}

	mux := http.NewServeMux()
	connServer := connect.NewServer()
	workoutexecutionv1serviceconnect.RegisterAdminWorkoutExecutionServiceHandler(connServer, mockHandler)
	connecthttp.Mount(mux, connServer)
	server := httptest.NewServer(mux)
	defer server.Close()

	reader := adapters.NewConnectWorkoutSessionReader(server.URL, "test-secret", server.Client())

	// Found PR
	logs, err := reader.GetSetLogs(context.Background(), "user-456", "bench-press", 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, 100.0, logs[0].Weight)
	assert.Equal(t, 5, logs[0].Reps)

	// No PR found -> returns empty slice, not error
	emptyLogs, errEmpty := reader.GetSetLogs(context.Background(), "user-456", "squat", 10)
	require.NoError(t, errEmpty)
	assert.Empty(t, emptyLogs)

	// Empty params
	emptyParamLogs, errParam := reader.GetSetLogs(context.Background(), "", "", 10)
	require.NoError(t, errParam)
	assert.Empty(t, emptyParamLogs)
}

func TestConnectReaders_ConstructorsAndErrors(t *testing.T) {
	ctx := context.Background()

	// New with nil http client
	pr := adapters.NewConnectUserProfileReader("http://127.0.0.1:54321", "sec", nil)
	assert.NotNil(t, pr)
	er := adapters.NewConnectExerciseCatalogReader("http://127.0.0.1:54321", "sec", nil)
	assert.NotNil(t, er)
	wr := adapters.NewConnectWorkoutSessionReader("http://127.0.0.1:54321", "sec", nil)
	assert.NotNil(t, wr)

	// Network connection failure returns error
	_, errP := pr.GetProfile(ctx, "user-err")
	assert.Error(t, errP)

	_, errE := er.SearchByFilter(ctx, nil)
	assert.Error(t, errE)

	_, errE2 := er.GetByID(ctx, "bench")
	assert.Error(t, errE2)

	_, errW := wr.GetRecentSessions(ctx, "user-err", time.Time{})
	assert.Error(t, errW)

	_, errW2 := wr.GetSetLogs(ctx, "user-err", "bench", 5)
	assert.Error(t, errW2)
}

func TestConnectReaders_EdgeCasesAndFallbacks(t *testing.T) {
	ctx := context.Background()

	// 1. UserProfileReader edge cases (DOB age calculation, empty defaults)
	profileHandler := &mockProfileHandler{
		getProfileFn: func(_ context.Context, _ *profilemsg.GetProfileRequest) (*profilemsg.GetProfileResponse, error) {
			return &profilemsg.GetProfileResponse{
				UserId:      "u-defaults",
				WeightKg:    0,
				HeightCm:    0,
				DateOfBirth: "2000-01-01",
				Age:         0,
			}, nil
		},
	}
	pMux := http.NewServeMux()
	pServerConn := connect.NewServer()
	profilev1serviceconnect.RegisterProfileServiceHandler(pServerConn, profileHandler)
	connecthttp.Mount(pMux, pServerConn)
	pServer := httptest.NewServer(pMux)
	defer pServer.Close()

	pReader := adapters.NewConnectUserProfileReader(pServer.URL, "secret", pServer.Client())
	p, err := pReader.GetProfile(ctx, "u-defaults")
	require.NoError(t, err)
	assert.Equal(t, 75.0, p.WeightKg)
	assert.Equal(t, 170.0, p.HeightCm)
	assert.Greater(t, p.Age, 20)
	assert.Equal(t, "hypertrophy", p.PrimaryGoal)
	assert.NotEmpty(t, p.AvailableEquipment)
	assert.NotEmpty(t, p.PreferredMuscleGroups)
	assert.NotEmpty(t, p.AvailableSlots)

	// 2. ExerciseCatalogReader edge cases (body part fallback, equipment filter)
	exerciseHandler := &mockExerciseHandler{
		searchFn: func(_ context.Context, req *exercisemsg.SearchExercisesRequest) (*exercisemsg.SearchExercisesResponse, error) {
			return &exercisemsg.SearchExercisesResponse{
				Exercises: []*exercisemsg.ExerciseInfo{
					{
						Id:          "lat-pull",
						Name:        "Lat Pulldown",
						BodyPartId:  "back",
						EquipmentId: "cable",
					},
					{
						Id:          "pull-up",
						Name:        "Pull Up",
						BodyPartId:  "back",
						EquipmentId: "pull-up-bar",
					},
				},
			}, nil
		},
	}
	eMux := http.NewServeMux()
	eServerConn := connect.NewServer()
	exercisev1serviceconnect.RegisterExerciseServiceHandler(eServerConn, exerciseHandler)
	connecthttp.Mount(eMux, eServerConn)
	eServer := httptest.NewServer(eMux)
	defer eServer.Close()

	eReader := adapters.NewConnectExerciseCatalogReader(eServer.URL, "secret", eServer.Client())
	exs, err := eReader.SearchByFilter(ctx, &port.ExerciseFilter{
		EquipmentIDs: []string{"cable", "pull-up-bar"},
		Limit:        10,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, exs, 2)
	assert.Equal(t, "back", exs[0].MuscleGroup)
	assert.True(t, exs[0].IsMachineOrCable)
	assert.True(t, exs[1].IsBodyweight)

	// 3. WorkoutSessionReader CodeNotFound returns empty slices
	workoutHandler := &mockWorkoutHandler{
		historyFn: func(_ context.Context, _ *workoutexecutionmsg.AdminGetWorkoutHistoryRequest) (*workoutexecutionmsg.AdminGetWorkoutHistoryResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, "not found")
		},
		prFn: func(_ context.Context, _ *workoutexecutionmsg.AdminGetPersonalRecordsRequest) (*workoutexecutionmsg.AdminGetPersonalRecordsResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, "not found")
		},
	}
	wMux := http.NewServeMux()
	wServerConn := connect.NewServer()
	workoutexecutionv1serviceconnect.RegisterAdminWorkoutExecutionServiceHandler(wServerConn, workoutHandler)
	connecthttp.Mount(wMux, wServerConn)
	wServer := httptest.NewServer(wMux)
	defer wServer.Close()

	wReader := adapters.NewConnectWorkoutSessionReader(wServer.URL, "secret", wServer.Client())
	sessions, err := wReader.GetRecentSessions(ctx, "user-404", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, sessions)

	logs, err := wReader.GetSetLogs(ctx, "user-404", "bench", 5)
	require.NoError(t, err)
	assert.Empty(t, logs)
}
