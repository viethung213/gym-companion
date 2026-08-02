package transport_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockSessionRepo struct {
	session  *aggregate.WorkoutSession
	sessions []*aggregate.WorkoutSession
	err      error
}

var _ repository.WorkoutSessionRepository = (*mockSessionRepo)(nil)

func (m *mockSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	return m.err
}

func (m *mockSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindTimedOutSessions(ctx context.Context, maxDurationMinutes int) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

func (m *mockSessionRepo) FindSessionsWithCriticalInactivity(ctx context.Context, threshold time.Duration) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

func (m *mockSessionRepo) FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.sessions != nil {
		return m.sessions, nil
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) GetRecentVolumesForMuscleGroup(ctx context.Context, userID, muscleGroup string, limit int) ([]float32, error) {
	return nil, nil
}

type mockPRRepo struct {
	records []*aggregate.PersonalRecord
	err     error
}

var _ repository.PersonalRecordRepository = (*mockPRRepo)(nil)

func (m *mockPRRepo) Save(ctx context.Context, pr *aggregate.PersonalRecord) error {
	return m.err
}

func (m *mockPRRepo) FindByUserIDAndExerciseID(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	return nil, m.err
}

func (m *mockPRRepo) FindByUserIDAndExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.records, nil
}

type mockMotionRepo struct {
	spec *aggregate.MotionSpecification
	err  error
}

var _ repository.MotionSpecificationRepository = (*mockMotionRepo)(nil)

func (m *mockMotionRepo) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {
	return m.err
}

func (m *mockMotionRepo) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.spec, nil
}

func (m *mockMotionRepo) Delete(ctx context.Context, exerciseID string) error {
	return m.err
}

func (m *mockMotionRepo) List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.spec != nil {
		return []*aggregate.MotionSpecification{m.spec}, 1, nil
	}
	return nil, 0, nil
}

type mockTxManager struct {
	err error
}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

func TestGRPCHandler(t *testing.T) {
	// Setup handlers
	sessionRepo := &mockSessionRepo{}
	prRepo := &mockPRRepo{}
	motionRepo := &mockMotionRepo{}
	tx := &mockTxManager{}

	startH := command.NewStartWorkoutSessionHandler(sessionRepo, nil, nil, tx)
	startScheduledH := command.NewStartScheduledWorkoutSessionHandler(sessionRepo, nil, tx)
	logSetH := command.NewLogWorkoutSetHandler(sessionRepo, nil, tx)
	completeH := command.NewCompleteWorkoutSessionHandler(sessionRepo, nil, nil, nil, tx)
	abortH := command.NewAbortWorkoutSessionHandler(sessionRepo, nil, tx)
	syncH := command.NewSyncWorkoutLogsHandler(sessionRepo, nil, tx)

	motionQ := query.NewGetMotionSpecificationQueryHandler(motionRepo)
	prQ := query.NewGetPersonalRecordsQueryHandler(prRepo)
	errsQ := query.NewGetWorkoutSessionErrorsQueryHandler(sessionRepo)
	historyQ := query.NewGetWorkoutHistoryQueryHandler(sessionRepo)

	grpcHandler := transport.NewGRPCHandler(
		startH, startScheduledH, logSetH, completeH, abortH, syncH,
		motionQ, prQ, errsQ, historyQ,
		command.NewUpdateMotionSpecificationHandler(motionRepo, nil, tx),
		command.NewDeleteMotionSpecificationHandler(motionRepo),
		query.NewListMotionSpecificationsQueryHandler(motionRepo),
		query.NewGetPresignedUploadURLQueryHandler(&mockStorageProvider{}),
		command.NewPatchMotionSpecificationAssetHandler(motionRepo, &mockStorageProvider{}, nil, tx),
	)

	t.Run("StartWorkoutSession error and success", func(t *testing.T) {
		// Error case
		_, err := grpcHandler.StartWorkoutSession(context.Background(), &workoutexecutionv1message.StartWorkoutSessionRequest{})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		// Success case with context UserID
		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "u1")
		res, err := grpcHandler.StartWorkoutSession(ctx, &workoutexecutionv1message.StartWorkoutSessionRequest{
			PlanId: "p1",
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetSessionId() == "" {
			t.Errorf("got empty SessionId")
		}
	})

	t.Run("StartScheduledWorkoutSession error and success", func(t *testing.T) {
		// Error case (unauthenticated)
		_, err := grpcHandler.StartScheduledWorkoutSession(context.Background(), &workoutexecutionv1message.StartScheduledWorkoutSessionRequest{})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		// Success case with context UserID
		scheduled, _ := aggregate.NewScheduledWorkoutSession("s1", "u1", "p1", time.Now().UTC())
		sessionRepo.session = scheduled

		ctx := context.WithValue(context.Background(), middleware.UserIDKey, "u1")
		res, err := grpcHandler.StartScheduledWorkoutSession(ctx, &workoutexecutionv1message.StartScheduledWorkoutSessionRequest{
			SessionId: "s1",
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetSessionId() == "" {
			t.Errorf("got empty SessionId")
		}
	})

	t.Run("LogWorkoutSet error and success", func(t *testing.T) {
		// Error
		_, err := grpcHandler.LogWorkoutSet(context.Background(), &workoutexecutionv1message.LogWorkoutSetRequest{})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		// Success
		session, _ := aggregate.NewWorkoutSession("sess-1", "u1", "p1")
		sessionRepo.session = session
		formScore := float32(90.0)

		res, err := grpcHandler.LogWorkoutSet(context.Background(), &workoutexecutionv1message.LogWorkoutSetRequest{
			SessionId:   "sess-1",
			SetNumber:   1,
			ExerciseId:  "ex1",
			TargetReps:  10,
			ActualReps:  10,
			Weight:      50.0,
			FormScore:   &formScore,
			Rpe:         8.0,
			CameraAngle: "front",
			Reps: []*workoutexecutionv1message.RepLog{
				{RepNumber: 1, RomPercentage: 90.0},
			},
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetSetLogId() == "" {
			t.Errorf("got empty SetLogId")
		}
	})

	t.Run("AbortWorkoutSession error and success", func(t *testing.T) {
		sessionRepo.session = nil
		_, err := grpcHandler.AbortWorkoutSession(context.Background(), &workoutexecutionv1message.AbortWorkoutSessionRequest{SessionId: "sess-1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		session, _ := aggregate.NewWorkoutSession("sess-1", "u1", "p1")
		sessionRepo.session = session
		res, err := grpcHandler.AbortWorkoutSession(context.Background(), &workoutexecutionv1message.AbortWorkoutSessionRequest{SessionId: "sess-1", Reason: "stop"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetSessionId() != "sess-1" {
			t.Errorf("got %v, want sess-1", res.GetSessionId())
		}
	})

	t.Run("CompleteWorkoutSession error and success", func(t *testing.T) {
		sessionRepo.session = nil
		_, err := grpcHandler.CompleteWorkoutSession(context.Background(), &workoutexecutionv1message.CompleteWorkoutSessionRequest{SessionId: "sess-1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		session, _ := aggregate.NewWorkoutSession("sess-1", "u1", "p1")
		sessionRepo.session = session
		res, err := grpcHandler.CompleteWorkoutSession(context.Background(), &workoutexecutionv1message.CompleteWorkoutSessionRequest{SessionId: "sess-1"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetSessionId() != "sess-1" {
			t.Errorf("got %v, want sess-1", res.GetSessionId())
		}
	})

	t.Run("SyncWorkoutLogs error and success", func(t *testing.T) {
		sessionRepo.session = nil
		_, err := grpcHandler.SyncWorkoutLogs(context.Background(), &workoutexecutionv1message.SyncWorkoutLogsRequest{SessionId: "sess-1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		session, _ := aggregate.NewWorkoutSession("sess-1", "u1", "p1")
		sessionRepo.session = session
		now := timestamppb.Now()
		_, err = grpcHandler.SyncWorkoutLogs(context.Background(), &workoutexecutionv1message.SyncWorkoutLogsRequest{
			SessionId: "sess-1",
			Errors: []*workoutexecutionv1message.ErrorLog{
				{ErrorCode: "ERR1", Severity: "HIGH", Timestamp: now},
			},
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("GetMotionSpecification error and success", func(t *testing.T) {
		motionRepo.spec = nil
		_, err := grpcHandler.GetMotionSpecification(context.Background(), &workoutexecutionv1message.GetMotionSpecificationRequest{ExerciseId: "ex1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		motionRepo.spec = aggregate.RestoreMotionSpecification("ex1", "http://detector.onnx", "http://skeleton.onnx", "http://rules", "http://dialogue", "front", true, time.Now().UTC(), time.Now().UTC())
		res, err := grpcHandler.GetMotionSpecification(context.Background(), &workoutexecutionv1message.GetMotionSpecificationRequest{ExerciseId: "ex1"})

		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.GetExerciseId() != "ex1" {
			t.Errorf("got %v, want ex1", res.GetExerciseId())
		}
	})

	t.Run("GetPersonalRecords error and success", func(t *testing.T) {
		ctxUser := context.WithValue(context.Background(), middleware.UserIDKey, "u1")
		prRepo.err = errors.New("db error")
		_, err := grpcHandler.GetPersonalRecords(ctxUser, &workoutexecutionv1message.GetPersonalRecordsRequest{ExerciseIds: []string{"ex1"}})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		prRepo.err = nil
		now := time.Now().UTC()
		prRepo.records = []*aggregate.PersonalRecord{
			aggregate.ReconstitutePersonalRecord("pr1", "u1", "ex1", 100, 100, 10, true, now, now, now),
		}
		res, err := grpcHandler.GetPersonalRecords(ctxUser, &workoutexecutionv1message.GetPersonalRecordsRequest{ExerciseIds: []string{"ex1"}})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if len(res.GetRecords()) != 1 {
			t.Errorf("got count = %d, want 1", len(res.GetRecords()))
		}
	})

	t.Run("GetWorkoutSessionErrors error and success", func(t *testing.T) {
		sessionRepo.session = nil
		_, err := grpcHandler.GetWorkoutSessionErrors(context.Background(), &workoutexecutionv1message.GetWorkoutSessionErrorsRequest{SessionId: "sess-1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		session, _ := aggregate.NewWorkoutSession("sess-1", "u1", "p1")
		session.AddErrors([]aggregate.SessionError{{ErrorCode: "ERR1"}})
		sessionRepo.session = session

		res, err := grpcHandler.GetWorkoutSessionErrors(context.Background(), &workoutexecutionv1message.GetWorkoutSessionErrorsRequest{SessionId: "sess-1"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if len(res.GetErrors()) != 1 {
			t.Errorf("got count = %d, want 1", len(res.GetErrors()))
		}
	})

	t.Run("GetWorkoutHistory error and success", func(t *testing.T) {
		ctxUser := context.WithValue(context.Background(), middleware.UserIDKey, "u1")
		sessionRepo.err = errors.New("db error")
		_, err := grpcHandler.GetWorkoutHistory(ctxUser, &workoutexecutionv1message.GetWorkoutHistoryRequest{Limit: 10, Offset: 0})
		if err == nil {
			t.Fatal("got nil, want error")
		}

		sessionRepo.err = nil
		now := time.Now().UTC()
		sessStarted := aggregate.ReconstituteWorkoutSession("sess-1", "u1", "p1", aggregate.StatusInProgress, nil, nil, nil, &now, nil, now, now)
		sessUnstarted := aggregate.ReconstituteWorkoutSession("sess-2", "u1", "p1", aggregate.StatusScheduled, nil, nil, nil, nil, nil, now, now)

		sessionRepo.sessions = []*aggregate.WorkoutSession{sessStarted, sessUnstarted}

		res, err := grpcHandler.GetWorkoutHistory(ctxUser, &workoutexecutionv1message.GetWorkoutHistoryRequest{Limit: 10, Offset: 0})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if len(res.GetSessions()) != 2 {
			t.Errorf("got count = %d, want 2", len(res.GetSessions()))
		}
	})

	t.Run("AdminGetPersonalRecords forbidden for User and success for Admin", func(t *testing.T) {
		ctxUser := context.WithValue(context.WithValue(context.Background(), middleware.UserIDKey, "u1"), middleware.UserRoleKey, "User")
		ctxAdmin := context.WithValue(context.WithValue(context.Background(), middleware.UserIDKey, "admin1"), middleware.UserRoleKey, "Admin")

		// User attempt -> Forbidden
		_, err := grpcHandler.AdminGetPersonalRecords(ctxUser, &workoutexecutionv1message.AdminGetPersonalRecordsRequest{UserId: "u1"})
		if err == nil {
			t.Fatal("expected forbidden error for User calling Admin endpoint, got nil")
		}

		// Admin attempt -> Success
		now := time.Now().UTC()
		prRepo.records = []*aggregate.PersonalRecord{
			aggregate.ReconstitutePersonalRecord("pr1", "u1", "ex1", 100, 100, 10, true, now, now, now),
		}
		res, err := grpcHandler.AdminGetPersonalRecords(ctxAdmin, &workoutexecutionv1message.AdminGetPersonalRecordsRequest{UserId: "u1"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if len(res.GetRecords()) != 1 {
			t.Errorf("got count = %d, want 1", len(res.GetRecords()))
		}
	})

	t.Run("AdminGetWorkoutHistory forbidden for User and success for Coach", func(t *testing.T) {
		ctxUser := context.WithValue(context.WithValue(context.Background(), middleware.UserIDKey, "u1"), middleware.UserRoleKey, "User")
		ctxCoach := context.WithValue(context.WithValue(context.Background(), middleware.UserIDKey, "coach1"), middleware.UserRoleKey, "Coach")

		// User attempt -> Forbidden
		_, err := grpcHandler.AdminGetWorkoutHistory(ctxUser, &workoutexecutionv1message.AdminGetWorkoutHistoryRequest{UserId: "u1", Limit: 10})
		if err == nil {
			t.Fatal("expected forbidden error for User calling Admin endpoint, got nil")
		}

		// Coach attempt -> Success
		now := time.Now().UTC()
		sessStarted := aggregate.ReconstituteWorkoutSession("sess-1", "u1", "p1", aggregate.StatusInProgress, nil, nil, nil, &now, nil, now, now)
		sessionRepo.sessions = []*aggregate.WorkoutSession{sessStarted}

		res, err := grpcHandler.AdminGetWorkoutHistory(ctxCoach, &workoutexecutionv1message.AdminGetWorkoutHistoryRequest{UserId: "u1", Limit: 10})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if len(res.GetSessions()) != 1 {
			t.Errorf("got count = %d, want 1", len(res.GetSessions()))
		}
	})
}

func TestLogWorkoutSet_ErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		wantCode codes.Code
	}{
		{
			name:     "ErrSessionNotInProgress maps to FailedPrecondition",
			repoErr:  derror.ErrSessionNotInProgress,
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "ErrSessionAlreadyCompleted maps to FailedPrecondition",
			repoErr:  derror.ErrSessionAlreadyCompleted,
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "ErrActiveSessionAlreadyExists maps to FailedPrecondition",
			repoErr:  derror.ErrActiveSessionAlreadyExists,
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "ErrWorkoutSessionNotFound maps to NotFound",
			repoErr:  derror.ErrWorkoutSessionNotFound,
			wantCode: codes.NotFound,
		},
		{
			name:     "unknown error maps to Internal",
			repoErr:  errors.New("unexpected db failure"),
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSessionRepo{err: tt.repoErr}
			tx := &mockTxManager{}
			prRepo := &mockPRRepo{}
			motionRepo := &mockMotionRepo{}

			startH := command.NewStartWorkoutSessionHandler(repo, nil, nil, tx)
			startScheduledH := command.NewStartScheduledWorkoutSessionHandler(repo, nil, tx)
			logSetH := command.NewLogWorkoutSetHandler(repo, nil, tx)
			completeH := command.NewCompleteWorkoutSessionHandler(repo, nil, nil, nil, tx)
			abortH := command.NewAbortWorkoutSessionHandler(repo, nil, tx)
			syncH := command.NewSyncWorkoutLogsHandler(repo, nil, tx)
			motionQ := query.NewGetMotionSpecificationQueryHandler(motionRepo)
			prQ := query.NewGetPersonalRecordsQueryHandler(prRepo)
			errsQ := query.NewGetWorkoutSessionErrorsQueryHandler(repo)
			historyQ := query.NewGetWorkoutHistoryQueryHandler(repo)

			h := transport.NewGRPCHandler(
				startH, startScheduledH, logSetH, completeH, abortH, syncH,
				motionQ, prQ, errsQ, historyQ,
				command.NewUpdateMotionSpecificationHandler(motionRepo, nil, tx),
				command.NewDeleteMotionSpecificationHandler(motionRepo),
				query.NewListMotionSpecificationsQueryHandler(motionRepo),
				query.NewGetPresignedUploadURLQueryHandler(&mockStorageProvider{}),
				command.NewPatchMotionSpecificationAssetHandler(motionRepo, &mockStorageProvider{}, nil, tx),
			)

			_, err := h.LogWorkoutSet(context.Background(), &workoutexecutionv1message.LogWorkoutSetRequest{
				SessionId:  "sess-1",
				ExerciseId: "ex-1",
				SetNumber:  1,
			})
			if err == nil {
				t.Fatal("got nil, want error")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("want status error, got %v", err)
			}
			if st.Code() != tt.wantCode {
				t.Errorf("got code %v, want %v", st.Code(), tt.wantCode)
			}
		})
	}
}

type mockStorageProvider struct{}

func (m *mockStorageProvider) UploadFile(ctx context.Context, fileName, contentType string, content io.Reader) (string, error) {
	return "https://cdn.fitai.com/models/" + fileName, nil
}

func (m *mockStorageProvider) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (string, string, error) {
	return "https://cdn.fitai.com/upload/" + fileName, "https://cdn.fitai.com/models/" + fileName, nil
}

func (m *mockStorageProvider) GetObject(ctx context.Context, objectKey string) ([]byte, error) {
	return []byte("{}"), nil
}

func (m *mockStorageProvider) PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	return "https://cdn.fitai.com/" + objectKey, nil
}
