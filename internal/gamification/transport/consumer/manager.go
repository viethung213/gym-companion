package consumer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	appService "github.com/viethung213/gym-companion/internal/gamification/application/service"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates all Kafka consumers for gamification.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	userRegisteredConsumer      *UserRegisteredConsumer
	workoutEventConsumer        *WorkoutEventConsumer
	personalRecordConsumer      *PersonalRecordConsumer
	nutritionEventConsumer      *NutritionEventConsumer
	userIdentityUpdatedConsumer *UserIdentityUpdatedConsumer
	wg                          sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	outboxLogRepo port.OutboxLogRepository,
	initUserGamificationHandler *command.InitUserGamificationHandler,
	coordinator *appService.ActivityIngestionCoordinator,
	prHandler PersonalRecordHandler,
	nutritionHandler NutritionActivityHandler,
	syncUserSnapshotHandler *command.SyncUserSnapshotHandler,
) *ConsumerManager {
	if registry == nil {
		return nil
	}

	mgr := &ConsumerManager{}

	dlqWriter, err := registry.GetDLQWriter()
	if err != nil {
		slog.Warn("Failed to obtain gamification DLQ writer", slog.Any("error", err))
	}

	if authReader, err := registry.GetReader("gamification-user-registered-group", "auth.events"); err == nil && authReader != nil {
		mgr.userRegisteredConsumer = NewUserRegisteredConsumer(
			authReader,
			initUserGamificationHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain gamification auth reader", slog.Any("error", err))
	}

	if workoutReader, err := registry.GetReader("gamification-workout-group", "workout_execution.events"); err == nil && workoutReader != nil {
		mgr.workoutEventConsumer = NewWorkoutEventConsumer(
			workoutReader,
			coordinator,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain gamification workout reader", slog.Any("error", err))
	}

	if prReader, err := registry.GetReader("gamification-pr-group", "workout_execution.events"); err == nil && prReader != nil {
		mgr.personalRecordConsumer = NewPersonalRecordConsumer(
			prReader,
			prHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain gamification PR reader", slog.Any("error", err))
	}

	if nutritionReader, err := registry.GetReader("gamification-nutrition-group", "nutrition.events"); err == nil && nutritionReader != nil {
		mgr.nutritionEventConsumer = NewNutritionEventConsumer(
			nutritionReader,
			nutritionHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain gamification nutrition reader", slog.Any("error", err))
	}

	if profileReader, err := registry.GetReader("gamification-profile-group", "profile.events"); err == nil && profileReader != nil {
		mgr.userIdentityUpdatedConsumer = NewUserIdentityUpdatedConsumer(
			profileReader,
			syncUserSnapshotHandler,
			outboxLogRepo,
		).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Failed to obtain gamification profile reader", slog.Any("error", err))
	}

	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.userRegisteredConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.userRegisteredConsumer.Start(ctx)
		}()
	}
	if m.workoutEventConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.workoutEventConsumer.Start(ctx)
		}()
	}
	if m.personalRecordConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.personalRecordConsumer.Start(ctx)
		}()
	}
	if m.nutritionEventConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.nutritionEventConsumer.Start(ctx)
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
