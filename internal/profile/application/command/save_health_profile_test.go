package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type mockRepo struct {
	profiles map[string]*aggregate.UserProfile
}

func newMockRepo() *mockRepo {
	return &mockRepo{profiles: make(map[string]*aggregate.UserProfile)}
}

func (m *mockRepo) Save(ctx context.Context, p *aggregate.UserProfile) error {
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	p, ok := m.profiles[userID]
	if !ok {
		return nil, derror.ErrProfileNotFound
	}
	return p, nil
}

func (m *mockRepo) Update(ctx context.Context, p *aggregate.UserProfile) error {
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockRepo) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	p, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return p.PeriodicMetrics(), nil
}

func (m *mockRepo) FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error) {
	p, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return p.Injuries(), nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockEventPub struct {
	published []any
}

func (m *mockEventPub) PublishEvents(ctx context.Context, events []any) error {
	m.published = append(m.published, events...)
	return nil
}

func TestSaveHealthProfileHandler(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	handler := command.NewSaveHealthProfileHandler(repo, eventPub, txManager)

	cmd := command.SaveHealthProfileCommand{
		UserID:                "user-test-1",
		WeightKg:              70.0,
		HeightCm:              175.0,
		DateOfBirth:           "1998-05-15",
		Age:                   28,
		Gender:                "FEMALE",
		Goals:                 []string{"WEIGHT_LOSS"},
		ExperienceLevel:       "BEGINNER",
		PreferredWorkoutTimes: []string{"EVENING"},
		Injuries: []command.InjuryInput{
			{ID: "inj-1", MuscleGroup: "Shoulder", Severity: "MILD", Notes: "Minor pain"},
		},
	}

	res, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "user-test-1", res.UserID)
	assert.True(t, res.AICoachActivated)
	assert.GreaterOrEqual(t, res.CompletionRate, 80.0)
	assert.Len(t, eventPub.published, 2)

	saved, err := repo.FindByUserID(context.Background(), "user-test-1")
	require.NoError(t, err)
	assert.Equal(t, "user-test-1", saved.UserID())
	assert.Len(t, saved.Injuries(), 1)
}
