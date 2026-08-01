//go:build unit

package query_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/application/query"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type mockRepo struct {
	profiles map[string]*aggregate.UserProfile
	failRepo bool
}

func (m *mockRepo) Save(ctx context.Context, p *aggregate.UserProfile) error {
	if m.failRepo {
		return errors.New("db error")
	}
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	if m.failRepo {
		return nil, errors.New("db error")
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, derror.ErrProfileNotFound
	}
	return p, nil
}

func (m *mockRepo) Update(ctx context.Context, p *aggregate.UserProfile) error {
	if m.failRepo {
		return errors.New("db error")
	}
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	if m.failRepo {
		return nil, errors.New("db error")
	}
	p, ok := m.profiles[userID]
	if !ok || p.PeriodicMetrics() == nil {
		return []vo.PeriodicMetric{}, nil
	}
	return p.PeriodicMetrics(), nil
}

func (m *mockRepo) FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error) {
	if m.failRepo {
		return nil, errors.New("db error")
	}
	p, ok := m.profiles[userID]
	if !ok || p.Injuries() == nil {
		return []*entity.Injury{}, nil
	}
	return p.Injuries(), nil
}

func TestQueryHandlers(t *testing.T) {
	repo := &mockRepo{profiles: make(map[string]*aggregate.UserProfile)}
	ctx := context.Background()

	bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile("user-q1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, p))

	t.Run("GetProfileHandler success & error", func(t *testing.T) {
		h := query.NewGetProfileHandler(repo)
		res, err := h.Handle(ctx, query.GetProfileQuery{UserID: "user-q1"})
		require.NoError(t, err)
		assert.Equal(t, "user-q1", res.UserID())

		repo.failRepo = true
		_, err = h.Handle(ctx, query.GetProfileQuery{UserID: "user-q1"})
		assert.Error(t, err)
		repo.failRepo = false
	})

	t.Run("GetBodyMetricsHistoryHandler success & error", func(t *testing.T) {
		h := query.NewGetBodyMetricsHistoryHandler(repo)
		metrics, err := h.Handle(ctx, query.GetBodyMetricsHistoryQuery{UserID: "user-q1"})
		require.NoError(t, err)
		assert.NotNil(t, metrics)

		repo.failRepo = true
		_, err = h.Handle(ctx, query.GetBodyMetricsHistoryQuery{UserID: "user-q1"})
		assert.Error(t, err)
		repo.failRepo = false
	})

	t.Run("GetInjuryHistoryHandler success & error", func(t *testing.T) {
		h := query.NewGetInjuryHistoryHandler(repo)
		injuries, err := h.Handle(ctx, query.GetInjuryHistoryQuery{UserID: "user-q1"})
		require.NoError(t, err)
		assert.NotNil(t, injuries)

		repo.failRepo = true
		_, err = h.Handle(ctx, query.GetInjuryHistoryQuery{UserID: "user-q1"})
		assert.Error(t, err)
		repo.failRepo = false
	})
}
