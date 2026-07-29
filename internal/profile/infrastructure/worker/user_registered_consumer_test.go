package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/worker"
)

type mockUserRepo struct {
	profiles map[string]*aggregate.UserProfile
	failRepo bool
}

func (m *mockUserRepo) Save(ctx context.Context, p *aggregate.UserProfile) error {
	if m.failRepo {
		return errors.New("save repo fail")
	}
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockUserRepo) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	if m.failRepo {
		return nil, errors.New("find repo fail")
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, derror.ErrProfileNotFound
	}
	return p, nil
}

func (m *mockUserRepo) Update(ctx context.Context, p *aggregate.UserProfile) error {
	m.profiles[p.UserID()] = p
	return nil
}

func (m *mockUserRepo) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	return nil, nil
}

func (m *mockUserRepo) FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error) {
	return nil, nil
}

type mockOutboxLogRepo struct {
	processed map[string]bool
	failCheck bool
}

func (m *mockOutboxLogRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	if m.failCheck {
		return false, errors.New("check outbox fail")
	}
	return m.processed[eventID], nil
}

func (m *mockOutboxLogRepo) SaveLog(ctx context.Context, record *port.OutboxLogRecord) error {
	if m.processed == nil {
		m.processed = make(map[string]bool)
	}
	m.processed[record.EventID] = true
	return nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestUserRegisteredConsumer_ProcessMessage(t *testing.T) {
	userRepo := &mockUserRepo{profiles: make(map[string]*aggregate.UserProfile)}
	outboxLogRepo := &mockOutboxLogRepo{processed: make(map[string]bool)}
	txManager := &mockTxManager{}

	consumer := worker.NewUserRegisteredConsumer(nil, userRepo, outboxLogRepo, txManager)
	ctx := context.Background()

	// 1. Valid UserRegistered event
	cloudEventJSON := `{
		"specversion": "1.0",
		"id": "evt-123",
		"source": "auth-service",
		"type": "contracts.generic.auth.v1.userRegistered",
		"time": "2026-07-28T12:00:00Z",
		"datacontenttype": "application/json",
		"data": {
			"userId": "user-registered-999",
			"email": "test@example.com"
		}
	}`

	msg := kafka.Message{Value: []byte(cloudEventJSON)}
	err := consumer.ProcessMessage(ctx, msg)
	require.NoError(t, err)

	// Verify profile created
	p, err := userRepo.FindByUserID(ctx, "user-registered-999")
	require.NoError(t, err)
	assert.Equal(t, "user-registered-999", p.UserID())

	// 2. Idempotency test (duplicate event)
	err = consumer.ProcessMessage(ctx, msg)
	require.NoError(t, err)

	// 3. Invalid JSON envelope
	err = consumer.ProcessMessage(ctx, kafka.Message{Value: []byte("invalid-json")})
	assert.Error(t, err)

	// 4. Unrelated event type
	unrelatedJSON := `{"id":"evt-other","type":"SomeOtherEvent","data":{}}`
	err = consumer.ProcessMessage(ctx, kafka.Message{Value: []byte(unrelatedJSON)})
	require.NoError(t, err)

	// 5. Empty userId event
	emptyUserJSON := `{"id":"evt-empty","type":"UserRegistered","data":{"userId":""}}`
	err = consumer.ProcessMessage(ctx, kafka.Message{Value: []byte(emptyUserJSON)})
	require.NoError(t, err)

	// 6. Existing profile case
	cloudEventExisting := `{
		"specversion": "1.0",
		"id": "evt-existing-456",
		"type": "UserRegistered",
		"data": {"userId": "user-registered-999"}
	}`
	err = consumer.ProcessMessage(ctx, kafka.Message{Value: []byte(cloudEventExisting)})
	require.NoError(t, err)

	// 7. Outbox log check error & retry failure test
	outboxLogRepo.failCheck = true
	userRepo.failRepo = true
	failUserJSON := `{
		"specversion": "1.0",
		"id": "evt-fail-789",
		"type": "UserRegistered",
		"data": {"userId": "user-fail-101"}
	}`
	err = consumer.ProcessMessage(ctx, kafka.Message{Value: []byte(failUserJSON)})
	assert.Error(t, err)
}
