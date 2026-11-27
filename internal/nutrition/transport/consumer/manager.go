package consumer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates Kafka consumer loops for the nutrition module.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	profileEventConsumer *ProfileEventConsumer
	workoutEventConsumer *WorkoutEventConsumer
	wg                   sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	planRepo repository.NutritionPlanRepository,
	outboxLogRepo port.OutboxLogRepository,
	genPlanHdlr *command.GenerateDailyPlanHandler,
	allergyRepo repository.UserAllergyRepository,
	rebalanceWorkoutHdlr *command.RebalanceWorkoutNutritionHandler,
) *ConsumerManager {
	if registry == nil {
		return nil
	}
	mgr := &ConsumerManager{}
	dlqWriter, _ := registry.GetDLQWriter()

	if profileEventReader, err := registry.GetReader("nutrition-profile-updated-group", "profile.events"); err == nil && profileEventReader != nil {
		mgr.profileEventConsumer = NewProfileEventConsumer(profileEventReader, planRepo, outboxLogRepo, genPlanHdlr).
			WithUserAllergyRepository(allergyRepo).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Nutrition profile event reader error", slog.Any("error", err))
	}

	if workoutReader, err := registry.GetReader("nutrition-workout-completed-group", "workout_execution.events"); err == nil && workoutReader != nil {
		mgr.workoutEventConsumer = NewWorkoutEventConsumer(workoutReader, rebalanceWorkoutHdlr, outboxLogRepo).WithDLQWriter(dlqWriter)
	} else if err != nil {
		slog.Warn("Nutrition workout event reader error", slog.Any("error", err))
	}

	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.profileEventConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			_ = m.profileEventConsumer.Start(ctx)
		}()
	}
	if m.workoutEventConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			_ = m.workoutEventConsumer.Start(ctx)
		}()
	}
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
