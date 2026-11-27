package consumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates all Kafka consumer loops and background workers for workout_execution.
type ConsumerManager struct {
	exerciseReader          sharedKafka.MessageReader
	exerciseCreatedConsumer *ExerciseCreatedConsumer

	sessionCompletedReader sharedKafka.MessageReader
	prConsumer             *PREventConsumer

	soloMatchReader            sharedKafka.MessageReader
	soloMatchStartedConsumer   *SoloMatchStartedConsumer
	soloMatchCancelledConsumer *SoloMatchCancelledConsumer
	dlqWriter                  sharedKafka.MessageWriter

	wg sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	exerciseCreatedConsumer *ExerciseCreatedConsumer,
	prConsumer *PREventConsumer,
	soloMatchStartedConsumer *SoloMatchStartedConsumer,
	soloMatchCancelledConsumer *SoloMatchCancelledConsumer,
) *ConsumerManager {
	if registry == nil {
		return nil
	}

	mgr := &ConsumerManager{
		exerciseCreatedConsumer:    exerciseCreatedConsumer,
		prConsumer:                 prConsumer,
		soloMatchStartedConsumer:   soloMatchStartedConsumer,
		soloMatchCancelledConsumer: soloMatchCancelledConsumer,
	}

	if dlqWriter, err := registry.GetDLQWriter(); err == nil && dlqWriter != nil {
		mgr.dlqWriter = dlqWriter
	}

	if reader, err := registry.GetReader("workout_execution_exercise_consumer", "exercise.events"); err == nil && reader != nil {
		mgr.exerciseReader = reader
	} else if err != nil {
		slog.Warn("Failed to obtain exercise events reader", slog.String("error", err.Error()))
	}

	if reader, err := registry.GetReader("workout_execution_pr_consumer", "workout_execution.events"); err == nil && reader != nil {
		mgr.sessionCompletedReader = reader
	} else if err != nil {
		slog.Warn("Failed to obtain PR consumer reader", slog.String("error", err.Error()))
	}

	if reader, err := registry.GetReader("workout_execution_solo_match_consumer", "solo_match.events"); err == nil && reader != nil {
		mgr.soloMatchReader = reader
	} else if err != nil {
		slog.Warn("Failed to obtain solo match reader", slog.String("error", err.Error()))
	}

	return mgr
}

// Start begins consuming Kafka events across all configured topics.
func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}

	if m.exerciseReader != nil && m.exerciseCreatedConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("PANIC RECOVERED in WorkoutExecution ExerciseCreated consumer worker", slog.Any("panic", r))
				}
				if closer, ok := m.exerciseReader.(interface{ Close() error }); ok {
					_ = closer.Close()
				}
			}()

			sharedKafka.RunConsumerLoop(
				ctx,
				m.exerciseReader,
				sharedKafka.ConsumerConfig{
					Name:        "WorkoutExecutionExerciseConsumer",
					MaxRetries:  3,
					BaseBackoff: 100 * time.Millisecond,
					DLQWriter:   m.dlqWriter,
				},
				func(ctx context.Context, msg *kafka.Message) error {
					return m.exerciseCreatedConsumer.HandleMessage(ctx, msg.Value)
				},
			)
		}()
	}

	if m.sessionCompletedReader != nil && m.prConsumer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("PANIC RECOVERED in WorkoutExecution PREventConsumer worker", slog.Any("panic", r))
				}
				if closer, ok := m.sessionCompletedReader.(interface{ Close() error }); ok {
					_ = closer.Close()
				}
			}()

			sharedKafka.RunConsumerLoop(
				ctx,
				m.sessionCompletedReader,
				sharedKafka.ConsumerConfig{
					Name:        "WorkoutExecutionPRConsumer",
					MaxRetries:  3,
					BaseBackoff: 100 * time.Millisecond,
					DLQWriter:   m.dlqWriter,
				},
				func(ctx context.Context, msg *kafka.Message) error {
					return m.prConsumer.HandleMessage(ctx, msg.Value)
				},
			)
		}()
	}

	if m.soloMatchReader != nil && (m.soloMatchStartedConsumer != nil || m.soloMatchCancelledConsumer != nil) {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("PANIC RECOVERED in WorkoutExecution SoloMatch consumer worker", slog.Any("panic", r))
				}
				if closer, ok := m.soloMatchReader.(interface{ Close() error }); ok {
					_ = closer.Close()
				}
			}()

			sharedKafka.RunConsumerLoop(
				ctx,
				m.soloMatchReader,
				sharedKafka.ConsumerConfig{
					Name:        "WorkoutExecutionSoloMatchConsumer",
					MaxRetries:  3,
					BaseBackoff: 100 * time.Millisecond,
					DLQWriter:   m.dlqWriter,
				},
				func(ctx context.Context, msg *kafka.Message) error {
					var err1, err2 error
					if m.soloMatchStartedConsumer != nil {
						err1 = m.soloMatchStartedConsumer.HandleMessage(ctx, msg.Value)
					}
					if m.soloMatchCancelledConsumer != nil {
						err2 = m.soloMatchCancelledConsumer.HandleMessage(ctx, msg.Value)
					}
					if err1 != nil {
						return err1
					}
					return err2
				},
			)
		}()
	}
}

// Wait blocks until all active consumer goroutines finish.
func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
