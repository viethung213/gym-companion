package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/query"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

type mockMotionRepo struct {
	spec *aggregate.MotionSpecification

	err error
}

var _ repository.MotionSpecificationRepository = (*mockMotionRepo)(nil)

func (m *mockMotionRepo) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {

	return nil

}

func (m *mockMotionRepo) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {

	if m.err != nil {

		return nil, m.err

	}

	return m.spec, nil

}

func (m *mockMotionRepo) Delete(ctx context.Context, exerciseID string) error {

	return nil

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

type mockPRRepo struct {
	records []*aggregate.PersonalRecord

	err error
}

var _ repository.PersonalRecordRepository = (*mockPRRepo)(nil)

func (m *mockPRRepo) Save(ctx context.Context, pr *aggregate.PersonalRecord) error {

	return nil

}

func (m *mockPRRepo) FindByUserIDAndExerciseID(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {

	return nil, nil

}

func (m *mockPRRepo) FindByUserIDAndExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {

	if m.err != nil {

		return nil, m.err

	}

	return m.records, nil

}

type mockSessionRepo struct {
	session *aggregate.WorkoutSession

	sessions []*aggregate.WorkoutSession

	err error
}

var _ repository.WorkoutSessionRepository = (*mockSessionRepo)(nil)

func (m *mockSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {

	return nil

}

func (m *mockSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {

	if m.err != nil {

		return nil, m.err

	}

	return m.session, nil

}

func (m *mockSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {

	return nil, nil

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

	return m.sessions, nil

}

func (m *mockSessionRepo) GetRecentVolumesForMuscleGroup(ctx context.Context, userID, muscleGroup string, limit int) ([]float32, error) {

	return nil, nil

}

func TestGetMotionSpecificationQueryHandler(t *testing.T) {

	t.Run("empty exercise ID", func(t *testing.T) {

		h := query.NewGetMotionSpecificationQueryHandler(&mockMotionRepo{})

		_, err := h.Handle(context.Background(), "", "coach-1")

		if !errors.Is(err, derror.ErrMotionSpecNotFound) {

			t.Errorf("got %v, want %v", err, derror.ErrMotionSpecNotFound)

		}

	})

	t.Run("repo error", func(t *testing.T) {

		h := query.NewGetMotionSpecificationQueryHandler(&mockMotionRepo{err: errors.New("db error")})

		_, err := h.Handle(context.Background(), "ex-1", "coach-1")

		if err == nil {

			t.Fatal("got nil, want error")

		}

	})

	t.Run("not found", func(t *testing.T) {

		h := query.NewGetMotionSpecificationQueryHandler(&mockMotionRepo{})

		_, err := h.Handle(context.Background(), "ex-1", "coach-1")

		if !errors.Is(err, derror.ErrMotionSpecNotFound) {

			t.Errorf("got %v, want %v", err, derror.ErrMotionSpecNotFound)

		}

	})

	t.Run("success", func(t *testing.T) {

		now := time.Now().UTC()

		spec := aggregate.RestoreMotionSpecification("ex-1", "http://detector.onnx", "http://skeleton.onnx", "http://rules.json", "http://dialogue.json", "front", true, now, now)

		h := query.NewGetMotionSpecificationQueryHandler(&mockMotionRepo{spec: spec})

		res, err := h.Handle(context.Background(), "ex-1", "coach-1")

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

		if res.ExerciseID() != "ex-1" {

			t.Errorf("got %v, want ex-1", res.ExerciseID())

		}

	})

}

func TestGetPersonalRecordsQueryHandler(t *testing.T) {

	t.Run("empty user ID", func(t *testing.T) {

		h := query.NewGetPersonalRecordsQueryHandler(&mockPRRepo{})

		_, err := h.Handle(context.Background(), "", []string{"ex-1"})

		if !errors.Is(err, derror.ErrPersonalRecordNotFound) {

			t.Errorf("got %v, want %v", err, derror.ErrPersonalRecordNotFound)

		}

	})

	t.Run("repo error", func(t *testing.T) {

		h := query.NewGetPersonalRecordsQueryHandler(&mockPRRepo{err: errors.New("db error")})

		_, err := h.Handle(context.Background(), "u-1", []string{"ex-1"})

		if err == nil {

			t.Fatal("got nil, want error")

		}

	})

	t.Run("success", func(t *testing.T) {

		now := time.Now().UTC()

		records := []*aggregate.PersonalRecord{

			aggregate.ReconstitutePersonalRecord("pr-1", "u-1", "ex-1", 100.0, 100.0, 10, true, now, now, now),
		}

		h := query.NewGetPersonalRecordsQueryHandler(&mockPRRepo{records: records})

		res, err := h.Handle(context.Background(), "u-1", []string{"ex-1"})

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

		if len(res) != 1 {

			t.Errorf("got count %d, want 1", len(res))

		}

	})

}

func TestGetWorkoutSessionErrorsQueryHandler(t *testing.T) {

	t.Run("session not found error", func(t *testing.T) {

		h := query.NewGetWorkoutSessionErrorsQueryHandler(&mockSessionRepo{})

		_, err := h.Handle(context.Background(), "sess-1")

		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {

			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)

		}

	})

	t.Run("success", func(t *testing.T) {

		session, _ := aggregate.NewWorkoutSession("sess-1", "u-1", "plan-1")

		session.AddErrors([]aggregate.SessionError{{ErrorCode: "ERR_1"}})

		h := query.NewGetWorkoutSessionErrorsQueryHandler(&mockSessionRepo{session: session})

		errs, err := h.Handle(context.Background(), "sess-1")

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

		if len(errs) != 1 {

			t.Errorf("got count %d, want 1", len(errs))

		}

	})

}

func TestGetWorkoutHistoryQueryHandler(t *testing.T) {

	t.Run("repo error", func(t *testing.T) {

		h := query.NewGetWorkoutHistoryQueryHandler(&mockSessionRepo{err: errors.New("db error")})

		_, err := h.Handle(context.Background(), "u-1", 0, -1)

		if err == nil {

			t.Fatal("got nil, want error")

		}

	})

	t.Run("success", func(t *testing.T) {

		session, _ := aggregate.NewWorkoutSession("sess-1", "u-1", "plan-1")

		h := query.NewGetWorkoutHistoryQueryHandler(&mockSessionRepo{sessions: []*aggregate.WorkoutSession{session}})

		res, err := h.Handle(context.Background(), "u-1", 10, 0)

		if err != nil {

			t.Fatalf("got err = %v, want nil", err)

		}

		if len(res) != 1 {

			t.Errorf("got count %d, want 1", len(res))

		}

	})

}

type mockStorageProvider struct {
	lastFileName string

	lastContentType string
}

func (m *mockStorageProvider) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (string, string, error) {

	m.lastFileName = fileName

	m.lastContentType = contentType

	return "http://upload.url/" + fileName, "http://cdn.url/" + fileName, nil

}

func (m *mockStorageProvider) GetObject(ctx context.Context, objectKey string) ([]byte, error) {

	return []byte("{}"), nil

}

func (m *mockStorageProvider) PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {

	return "http://cdn.url/" + objectKey, nil

}

func TestGetPresignedUploadURLQueryHandler(t *testing.T) {

	mockStorage := &mockStorageProvider{}

	handler := query.NewGetPresignedUploadURLQueryHandler(mockStorage)

	t.Run("empty filename returns error", func(t *testing.T) {

		_, err := handler.Handle(context.Background(), query.GetPresignedUploadURLQuery{FileName: "  "})

		if err == nil {

			t.Error("want error on empty filename, got nil")

		}

	})

	t.Run("path traversal returns error", func(t *testing.T) {

		_, err := handler.Handle(context.Background(), query.GetPresignedUploadURLQuery{FileName: "../secret.onnx"})

		if err == nil {

			t.Error("want error on path traversal '..', got nil")

		}

	})

	t.Run("flexible extension resolution for .yaml", func(t *testing.T) {

		res, err := handler.Handle(context.Background(), query.GetPresignedUploadURLQuery{

			FileName: "rules/squat_rule",

			ContentType: ".yaml",
		})

		if err != nil {

			t.Fatalf("unexpected error: %v", err)

		}

		if res.FileName != "rules/squat_rule.yaml" {

			t.Errorf("got res.FileName = %s, want rules/squat_rule.yaml", res.FileName)

		}

	})

	t.Run("flexible extension resolution for yaml without dot", func(t *testing.T) {

		res, err := handler.Handle(context.Background(), query.GetPresignedUploadURLQuery{

			FileName: "rules/squat_rule",

			ContentType: "yaml",
		})

		if err != nil {

			t.Fatalf("unexpected error: %v", err)

		}

		if res.FileName != "rules/squat_rule.yaml" {

			t.Errorf("got res.FileName = %s, want rules/squat_rule.yaml", res.FileName)

		}

	})

	t.Run("preserves exact fileName if extension already exists", func(t *testing.T) {

		res, err := handler.Handle(context.Background(), query.GetPresignedUploadURLQuery{

			FileName: "rules/squat_rule.json",

			ContentType: "application/json",
		})

		if err != nil {

			t.Fatalf("unexpected error: %v", err)

		}

		if res.FileName != "rules/squat_rule.json" {

			t.Errorf("got res.FileName = %s, want rules/squat_rule.json", res.FileName)

		}

	})

}
