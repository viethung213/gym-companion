package consumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates Kafka consumer loops for the exercise module.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	reader    sharedKafka.MessageReader
	dlqWriter sharedKafka.MessageWriter
	consumer  *MotionSpecConsumer
	wg        sync.WaitGroup
}

func NewConsumerManager(registry *sharedKafka.Registry, consumer *MotionSpecConsumer) *ConsumerManager {
	if registry == nil || consumer == nil {
		return nil
	}
	mgr := &ConsumerManager{consumer: consumer}
	if dlqWriter, err := registry.GetDLQWriter(); err == nil && dlqWriter != nil {
		mgr.dlqWriter = dlqWriter
	}
	reader, err := registry.GetReader("exercise_motion_spec_consumer", "workout_execution.events")
	if err == nil && reader != nil {
		mgr.reader = reader
	} else if err != nil {
		slog.Warn("Failed to obtain exercise motion spec reader", slog.String("error", err.Error()))
	}
	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil || m.reader == nil || m.consumer == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC RECOVERED in Exercise MotionSpec consumer worker", slog.Any("panic", r))
			}
			if closer, ok := m.reader.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}()

		sharedKafka.RunConsumerLoop(
			ctx,
			m.reader,
			sharedKafka.ConsumerConfig{
				Name:        "ExerciseMotionSpecConsumer",
				MaxRetries:  3,
				BaseBackoff: 100 * time.Millisecond,
				DLQWriter:   m.dlqWriter,
			},
			func(ctx context.Context, msg *segmentio.Message) error {
				return m.consumer.ConsumeMotionSpecReady(ctx, msg.Value)
			},
		)
	}()
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
