package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// Helper constructors for test mocks.
func newMockSessionRepo(tb testing.TB) *mockSessionRepo {
	tb.Helper()
	return &mockSessionRepo{}
}

func newMockPRRepo(tb testing.TB) *mockPRRepo {
	tb.Helper()
	return &mockPRRepo{}
}

func newMockTxManager(tb testing.TB) *mockTxManager {
	tb.Helper()
	return &mockTxManager{}
}

func newMockOutboxWriter(tb testing.TB) *mockOutboxWriter {
	tb.Helper()
	return &mockOutboxWriter{}
}

type mockSessionRepo struct {
	session       *aggregate.WorkoutSession
	activeSession *aggregate.WorkoutSession
	findErr       error
	saveErr       error
	historyErr    error
	timedOutErr   error
	recentVolsErr error
	recentVols    []float32
}

var _ repository.WorkoutSessionRepository = (*mockSessionRepo)(nil)

func (m *mockSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	return m.saveErr
}

func (m *mockSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.activeSession, nil
}

func (m *mockSessionRepo) FindTimedOutSessions(ctx context.Context, maxDurationMinutes int) ([]*aggregate.WorkoutSession, error) {
	if m.timedOutErr != nil {
		return nil, m.timedOutErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	if m.historyErr != nil {
		return nil, m.historyErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindSessionsWithCriticalInactivity(ctx context.Context, threshold time.Duration) ([]*aggregate.WorkoutSession, error) {
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) GetRecentVolumesForMuscleGroup(ctx context.Context, userID, muscleGroup string, limit int) ([]float32, error) {
	if m.recentVolsErr != nil {
		return nil, m.recentVolsErr
	}
	return m.recentVols, nil
}

type mockPRRepo struct {
	pr      *aggregate.PersonalRecord
	findErr error
	saveErr error
}

var _ repository.PersonalRecordRepository = (*mockPRRepo)(nil)

func (m *mockPRRepo) Save(ctx context.Context, pr *aggregate.PersonalRecord) error {
	return m.saveErr
}

func (m *mockPRRepo) FindByUserIDAndExerciseID(ctx context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.pr, nil
}

func (m *mockPRRepo) FindByUserIDAndExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.pr != nil {
		return []*aggregate.PersonalRecord{m.pr}, nil
	}
	return nil, nil
}

type mockPlanClient struct {
	exists bool
	err    error
}

func (m *mockPlanClient) ValidatePlanExists(ctx context.Context, userID, planID string) (bool, error) {
	return m.exists, m.err
}

type mockExerciseClient struct {
	group string
	err   error
}

func (m *mockExerciseClient) GetExerciseMuscleGroup(ctx context.Context, exerciseID string) (string, error) {
	return m.group, m.err
}

type mockUserClient struct {
	err error
}

func (m *mockUserClient) UpdateBodyWeight(ctx context.Context, userID string, weightKg float32) error {
	return m.err
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

type mockOutboxWriter struct {
	err error
}

func (m *mockOutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	return m.err
}
