package consumer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates Kafka consumer loops for the coaching module.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	profileConsumer *ProfileCompletedConsumer
	workoutConsumer *WorkoutExecutionConsumer
	wg              sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	initiateHandler *command.InitiateRoadmapHandler,
	regenerateHandler *command.RegenerateScheduleHandler,
	completeSessionHandler *command.CompleteSessionHandler,
	abortSessionHandler *command.AbortSessionHandler,
	outboxRepo port.OutboxRepository,
) *ConsumerManager {
	if registry == nil {
		return nil
	}
	mgr := &ConsumerManager{}
	dlqWriter, _ := registry.GetDLQWriter()

	if initiateHandler != nil && outboxRepo != nil {
		reader, err := registry.GetReader("coaching-profile-completed-group", "profile.events")
		if err == nil && reader != nil {
			mgr.profileConsumer = NewProfileCompletedConsumer(reader, initiateHandler, regenerateHandler, outboxRepo).
				WithDLQWriter(dlqWriter)
		} else if err != nil {
			slog.Warn("Coaching ProfileCompletedConsumer reader error", slog.Any("error", err))
		}
	}

	if completeSessionHandler != nil && abortSessionHandler != nil && outboxRepo != nil {
		reader, err := registry.GetReader("coaching-workout-execution-group", "workout_execution.events")
		if err == nil && reader != nil {
			mgr.workoutConsumer = NewWorkoutExecutionConsumer(reader, completeSessionHandler, abortSessionHandler, outboxRepo).
				WithDLQWriter(dlqWriter)
		} else if err != nil {
			slog.Warn("Coaching WorkoutExecutionConsumer reader error", slog.Any("error", err))
		}
	}

	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.profileConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.profileConsumer.Start(ctx)
		}()
	}
	if m.workoutConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.workoutConsumer.Start(ctx)
		}()
	}
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
