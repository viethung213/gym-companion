package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

// ConsumerManager coordinates all Kafka consumer loops for the notification module.
//
//nolint:revive // ConsumerManager is descriptive across modules
type ConsumerManager struct {
	consumers       []*NotificationEventConsumer
	primaryConsumer *NotificationEventConsumer
	wg              sync.WaitGroup
}

// NewConsumerManager initializes Kafka consumers across high-priority, normal-priority,
// and isolated core domain inbound topics.
func NewConsumerManager(
	registry *sharedKafka.Registry,
	sendPushHandler *command.SendPushNotificationHandler,
	sendEmailHandler *command.SendEmailNotificationHandler,
	sendSMSHandler *command.SendSMSNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
	deviceRepo repository.DeviceRepository,
) *ConsumerManager {
	if registry == nil {
		return nil
	}

	mgr := &ConsumerManager{
		consumers: make([]*NotificationEventConsumer, 0, 7),
	}

	dlqWriter, dlqErr := registry.GetDLQWriter()
	if dlqErr != nil {
		slog.Warn("Failed to get kafka writer for system DLQ topic", slog.Any("error", dlqErr))
	}

	createConsumer := func(reader sharedKafka.MessageReader) *NotificationEventConsumer {
		c := NewNotificationEventConsumer(
			reader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo,
		).WithDLQWriter(dlqWriter).WithDeviceRepo(deviceRepo)
		if mgr.primaryConsumer == nil {
			mgr.primaryConsumer = c
		}
		return c
	}

	// 1. HIGH-PRIORITY Notification Consumers: 3 independent Readers for 3 Partitions (Option B)
	highTopic := "notification.events.high-priority"
	const highWorkers = 3
	for i := 1; i <= highWorkers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		reader, rErr := registry.GetReaderInstance("notification-group-high", highTopic, workerID)
		if rErr == nil && reader != nil {
			c := createConsumer(reader)
			mgr.consumers = append(mgr.consumers, c)
		} else {
			slog.Warn("Failed to get kafka reader for high-priority worker",
				slog.String("worker", workerID),
				slog.String("topic", highTopic),
				slog.Any("error", rErr),
			)
		}
	}
	slog.Info("Notification HIGH-PRIORITY consumers started",
		slog.Int("workers", highWorkers),
		slog.String("topic", highTopic),
	)

	// 2. NORMAL-PRIORITY Notification Consumer: Dedicated group for background/standard dispatch
	normalTopic := "notification.events.normal-priority"
	normalReader, normalErr := registry.GetReader("notification-group-normal", normalTopic)
	if normalErr == nil && normalReader != nil {
		c := createConsumer(normalReader)
		mgr.consumers = append(mgr.consumers, c)
		slog.Info("Notification NORMAL-PRIORITY consumer started", slog.String("topic", normalTopic))
	} else {
		slog.Warn("Failed to get kafka reader for normal-priority topic",
			slog.String("topic", normalTopic),
			slog.Any("error", normalErr),
		)
	}

	// 3. CORE DOMAIN Inbound Event Consumers (Coaching, Nutrition, Workout Execution)
	// Each topic is isolated into its own Consumer Group to avoid Kafka Topology Conflict
	// and Rebalance Thrashing (BUG-01).
	coreTargets := []struct {
		topic   string
		groupID string
	}{
		{topic: "coaching.events", groupID: "notification-group-core-coaching"},
		{topic: "nutrition.events", groupID: "notification-group-core-nutrition"},
		{topic: "workout_execution.events", groupID: "notification-group-core-workout"},
	}

	for _, target := range coreTargets {
		reader, rErr := registry.GetReader(target.groupID, target.topic)
		if rErr == nil && reader != nil {
			c := createConsumer(reader)
			mgr.consumers = append(mgr.consumers, c)
			slog.Info("Notification Core consumer started",
				slog.String("topic", target.topic),
				slog.String("group_id", target.groupID),
			)
		} else {
			slog.Warn("Failed to get kafka reader for core topic",
				slog.String("topic", target.topic),
				slog.String("group_id", target.groupID),
				slog.Any("error", rErr),
			)
		}
	}

	// Inbound Event Failure Retry Worker fallback if no reader was initialized
	if mgr.primaryConsumer == nil {
		mgr.primaryConsumer = createConsumer(nil)
	}

	return mgr
}

// PrimaryConsumer returns the primary consumer used by background retry workers.
func (m *ConsumerManager) PrimaryConsumer() *NotificationEventConsumer {
	if m == nil {
		return nil
	}
	return m.primaryConsumer
}

// Start launches all configured consumer loops concurrently in separate goroutines.
func (m *ConsumerManager) Start(ctx context.Context) {
	if m == nil {
		return
	}

	for _, c := range m.consumers {
		m.wg.Add(1)
		go func(consumer *NotificationEventConsumer) {
			defer m.wg.Done()
			consumer.Start(ctx)
		}(c)
	}
}

// Wait blocks until all consumer loops exit cleanly.
func (m *ConsumerManager) Wait() {
	if m == nil {
		return
	}
	m.wg.Wait()
}
