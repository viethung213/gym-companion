package consumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// MessageHandler defines the contract for processing raw event payloads.
type MessageHandler interface {
	HandleMessage(ctx context.Context, rawPayload []byte) error
}

// KafkaMessageReader abstracts kafka.Reader for testing and modularity.
type KafkaMessageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// TopicConsumer handles consuming and dispatching messages for a specific topic.
type TopicConsumer struct {
	name      string
	reader    KafkaMessageReader
	dlqWriter sharedKafka.MessageWriter
	handlers  []MessageHandler
}

func NewTopicConsumer(name string, reader KafkaMessageReader, handlers ...MessageHandler) *TopicConsumer {
	return &TopicConsumer{
		name:     name,
		reader:   reader,
		handlers: handlers,
	}
}

func (w *TopicConsumer) WithDLQWriter(writer sharedKafka.MessageWriter) *TopicConsumer {
	w.dlqWriter = writer
	return w
}

func (w *TopicConsumer) Start(ctx context.Context) {
	if w.reader == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "PANIC RECOVERED in SoloMatch consumer", slog.String("worker", w.name), slog.Any("panic", r))
		}
		if closeErr := w.reader.Close(); closeErr != nil {
			slog.WarnContext(ctx, "Error closing SoloMatch reader", slog.String("worker", w.name), slog.String("error", closeErr.Error()))
		}
	}()

	sharedKafka.RunConsumerLoop(
		ctx,
		w.reader,
		sharedKafka.ConsumerConfig{
			Name:        w.name,
			MaxRetries:  3,
			BaseBackoff: 100 * time.Millisecond,
			DLQWriter:   w.dlqWriter,
		},
		func(ctx context.Context, msg *kafka.Message) error {
			return w.dispatchMessage(ctx, msg.Value)
		},
	)
}

func (w *TopicConsumer) dispatchMessage(ctx context.Context, rawPayload []byte) error {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "PANIC in processing event in SoloMatch consumer", slog.String("worker", w.name), slog.Any("panic", r))
		}
	}()

	var lastErr error
	for _, h := range w.handlers {
		if h == nil {
			continue
		}
		if err := h.HandleMessage(ctx, rawPayload); err != nil {
			slog.ErrorContext(ctx, "SoloMatch consumer handler error", slog.String("worker", w.name), slog.String("error", err.Error()))
			lastErr = err
		}
	}
	return lastErr
}

// ConsumerManager coordinates all topic consumers for solo_match.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	consumers []*TopicConsumer
	wg        sync.WaitGroup
}

func NewConsumerManager(
	registry *sharedKafka.Registry,
	workoutCompleted *WorkoutCompletedConsumer,
	workoutAborted *WorkoutAbortedConsumer,
	roomReady *SoloMatchRoomReadyConsumer,
	userRegistered *UserRegisteredConsumer,
	userIdentityUpdated *UserIdentityUpdatedConsumer,
	xpEarned *XpEarnedConsumer,
) *ConsumerManager {
	if registry == nil {
		return nil
	}

	mgr := &ConsumerManager{}
	dlqWriter, _ := registry.GetDLQWriter()

	// 1. Workout events
	if reader, err := registry.GetReader("solo_match_workout_consumer", "workout_execution.events"); err == nil && reader != nil {
		c := NewTopicConsumer("Workout Events Consumer", reader, workoutCompleted, workoutAborted, roomReady).WithDLQWriter(dlqWriter)
		mgr.consumers = append(mgr.consumers, c)
	} else if err != nil {
		slog.Warn("Failed to obtain workout events reader", slog.Any("error", err))
	}

	// 2. Auth events
	if reader, err := registry.GetReader("solo_match_user_consumer", "auth.events"); err == nil && reader != nil {
		mgr.consumers = append(mgr.consumers, NewTopicConsumer("Auth Events Consumer", reader, userRegistered).WithDLQWriter(dlqWriter))
	} else if err != nil {
		slog.Warn("Failed to obtain auth events reader", slog.Any("error", err))
	}

	// 3. Profile events
	if reader, err := registry.GetReader("solo_match_profile_consumer", "profile.events"); err == nil && reader != nil {
		mgr.consumers = append(mgr.consumers, NewTopicConsumer("Profile Events Consumer", reader, userIdentityUpdated).WithDLQWriter(dlqWriter))
	} else if err != nil {
		slog.Warn("Failed to obtain profile events reader", slog.Any("error", err))
	}

	// 4. Gamification events
	if reader, err := registry.GetReader("solo_match_gamification_consumer", "gamification.events"); err == nil && reader != nil {
		mgr.consumers = append(mgr.consumers, NewTopicConsumer("Gamification Events Consumer", reader, xpEarned).WithDLQWriter(dlqWriter))
	} else if err != nil {
		slog.Warn("Failed to obtain gamification events reader", slog.Any("error", err))
	}

	return mgr
}

func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	for _, c := range m.consumers {
		tc := c
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			tc.Start(ctx)
		}()
	}
}

func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
