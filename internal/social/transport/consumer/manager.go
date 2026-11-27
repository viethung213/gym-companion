package consumer

import (
	"context"
	"log/slog"
	"sync"

	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/port"
)

// ConsumerManager coordinates all Kafka consumers for social.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	workoutSharedConsumer       *WorkoutSharedConsumer
	userRegisteredConsumer      *UserRegisteredConsumer
	userRoleUpdatedConsumer     *UserRoleUpdatedConsumer
	userIdentityUpdatedConsumer *UserIdentityUpdatedConsumer
	wg                          sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	outboxLogRepo port.OutboxLogRepository,
	txManager port.TransactionManager,
	ingestWorkoutActivityHandler *command.IngestWorkoutActivityHandler,
	syncUserSnapshotHandler *command.SyncUserSnapshotHandler,
	updateUserRoleHandler *command.UpdateUserRoleHandler,
) *ConsumerManager {
	if registry == nil {
		return nil
	}
	mgr := &ConsumerManager{}

	dlqWriter, err := registry.GetDLQWriter()
	if err != nil {
		slog.Warn("Failed to obtain social DLQ writer", slog.Any("error", err))
	}

	if workoutReader, err := registry.GetReader("social-workout-shared-group", "workout_execution.events"); err == nil && workoutReader != nil {
		mgr.workoutSharedConsumer = NewWorkoutSharedConsumer(
			workoutReader,
			ingestWorkoutActivityHandler,
			outboxLogRepo,
			txManager,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain social workout shared reader", slog.Any("error", err))
	}

	if authReader, err := registry.GetReader("social-user-registered-group", "auth.events"); err == nil && authReader != nil {
		mgr.userRegisteredConsumer = NewUserRegisteredConsumer(
			authReader,
			syncUserSnapshotHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain social user registered reader", slog.Any("error", err))
	}

	if authRoleReader, err := registry.GetReader("social-user-role-updated-group", "auth.events"); err == nil && authRoleReader != nil {
		mgr.userRoleUpdatedConsumer = NewUserRoleUpdatedConsumer(
			authRoleReader,
			updateUserRoleHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain social user role reader", slog.Any("error", err))
	}

	if profileReader, err := registry.GetReader("social-user-identity-updated-group", "profile.events"); err == nil && profileReader != nil {
		mgr.userIdentityUpdatedConsumer = NewUserIdentityUpdatedConsumer(
			profileReader,
			syncUserSnapshotHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain social profile identity reader", slog.Any("error", err))
	}

	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.workoutSharedConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.workoutSharedConsumer.Start(ctx)
		}()
	}
	if m.userRegisteredConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.userRegisteredConsumer.Start(ctx)
		}()
	}
	if m.userRoleUpdatedConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.userRoleUpdatedConsumer.Start(ctx)
		}()
	}
	if m.userIdentityUpdatedConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.userIdentityUpdatedConsumer.Start(ctx)
		}()
	}
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
