package worker_test

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

type mockSessionRepo struct {
	session *aggregate.WorkoutSession
	findErr error
	saveErr error
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

func (m *mockSessionRepo) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.session, nil
}

func (m *mockSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	return m.session, m.findErr
}

func (m *mockSessionRepo) FindTimedOutSessions(ctx context.Context, timeoutMinutes int) ([]*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindSessionsWithCriticalInactivity(ctx context.Context, threshold time.Duration) ([]*aggregate.WorkoutSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.session != nil {
		return []*aggregate.WorkoutSession{m.session}, nil
	}
	return nil, nil
}

func (m *mockSessionRepo) FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
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
