package consumer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates all Kafka consumers for profile.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	userRegisteredConsumer *UserRegisteredConsumer
	wg                     sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	userRepo repository.UserProfileRepository,
	outboxLogRepo port.OutboxLogRepository,
	txManager port.TransactionManager,
) *ConsumerManager {
	if registry == nil {
		return nil
	}
	mgr := &ConsumerManager{}
	dlqWriter, _ := registry.GetDLQWriter()
	reader, err := registry.GetReader("profile-user-registered-group", "auth.events")
	if err == nil && reader != nil {
		mgr.userRegisteredConsumer = NewUserRegisteredConsumer(reader, userRepo, outboxLogRepo, txManager).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain profile auth reader", slog.Any("error", err))
	}
	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil || m.userRegisteredConsumer == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.userRegisteredConsumer.Start(ctx)
	}()
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
