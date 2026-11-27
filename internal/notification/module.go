package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/service/notificationv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
	notificationEvent "github.com/viethung213/gym-companion/internal/notification/infrastructure/event"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/fcm"
	notificationKafka "github.com/viethung213/gym-companion/internal/notification/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
	notificationWorker "github.com/viethung213/gym-companion/internal/notification/infrastructure/worker"
	notificationConsumer "github.com/viethung213/gym-companion/internal/notification/transport/consumer"
	notificationGRPC "github.com/viethung213/gym-companion/internal/notification/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/database"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*notificationGRPC.GRPCHandler, func(), error) {
	if deps.DB == nil {
		return nil, nil, errors.New("deps.DB is required")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("load notification config: %w", err)
	}

	// Wrap sql.DB with GORM with OpenTelemetry tracing
	gormDB, err := database.OpenGorm(deps.DB, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("wrap connection pool in gorm: %w", err)
	}

	// 1. Repositories & Adapters
	deviceRepo := persistence.NewDeviceRepository(gormDB)
	settingRepo := persistence.NewSettingRepository(gormDB)
	notificationRepo := persistence.NewNotificationRepository(gormDB)
	outboxRepo := persistence.NewOutboxRepository(gormDB)
	outboxLogRepo := persistence.NewOutboxLogRepository(gormDB)
	txManager := persistence.NewSQLTransactionManager(gormDB)
	fcmClient := fcm.NewClient(&cfg)
	emailProvider := email.NewEmailProvider(&cfg)
	smsProvider := sms.NewSMSProvider(&cfg)
	eventRecorder := notificationEvent.NewOutboxWriter(outboxRepo)

	// 2. Command Handlers
	registerDeviceHandler := command.NewRegisterDeviceTokenHandler(deviceRepo)
	sendPushHandler := command.NewSendPushNotificationHandler(deviceRepo, notificationRepo, settingRepo, fcmClient, txManager, eventRecorder)
	sendEmailHandler := command.NewSendEmailNotificationHandler(settingRepo, emailProvider)
	sendSMSHandler := command.NewSendSMSNotificationHandler(settingRepo, smsProvider)
	updateSettingsHandler := command.NewUpdateNotificationSettingsHandler(settingRepo)
	markAsReadHandler := command.NewMarkNotificationAsReadHandler(notificationRepo)

	// 3. Query Handlers
	getSettingsHandler := query.NewGetNotificationSettingsHandler(settingRepo)
	listNotifsHandler := query.NewListNotificationsHandler(notificationRepo)

	// 4. gRPC Transport Handler
	grpcHandler := notificationGRPC.NewGRPCHandler(
		sendPushHandler,
		registerDeviceHandler,
		updateSettingsHandler,
		markAsReadHandler,
		getSettingsHandler,
		listNotifsHandler,
	)

	// 5. Setup Outbox Worker, Inbound Retry Worker & Consumer in background if KafkaRegistry is provided
	var cancelWorkers context.CancelFunc
	ctxWorkers, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup

	var kafkaPub *notificationKafka.Publisher
	if deps.KafkaRegistry != nil {
		// Outbound Outbox Worker Publisher
		writer, wErr := deps.KafkaRegistry.GetWriter("notification.events")
		if wErr == nil && writer != nil {
			kafkaPub = notificationKafka.NewPublisher(writer)
			outboxWorker := notificationWorker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = outboxWorker.Start(ctxWorkers)
			}()
		} else {
			slog.Warn("Failed to get kafka writer for notification outbox", slog.Any("error", wErr))
		}

		dlqWriter, dlqErr := deps.KafkaRegistry.GetDLQWriter()
		if dlqErr != nil {
			slog.Warn("Failed to get kafka writer for system DLQ topic", slog.Any("error", dlqErr))
		}

		// 1. HIGH-PRIORITY Notification Consumers: 3 independent Readers for 3 Partitions (Option B)
		highTopic := "notification.events.high-priority"
		const highWorkers = 3
		var primaryConsumer *notificationConsumer.NotificationEventConsumer
		for i := 1; i <= highWorkers; i++ {
			workerID := fmt.Sprintf("worker-%d", i)
			reader, rErr := deps.KafkaRegistry.GetReaderInstance("notification-group-high", highTopic, workerID)
			if rErr == nil && reader != nil {
				consumer := notificationConsumer.NewNotificationEventConsumer(
					reader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo,
				).WithDLQWriter(dlqWriter).WithDeviceRepo(deviceRepo)
				if primaryConsumer == nil {
					primaryConsumer = consumer
				}
				wg.Add(1)
				go func(c *notificationConsumer.NotificationEventConsumer) {
					defer wg.Done()
					c.Start(ctxWorkers)
				}(consumer)
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
		normalReader, normalErr := deps.KafkaRegistry.GetReader("notification-group-normal", normalTopic)
		if normalErr == nil && normalReader != nil {
			normalConsumer := notificationConsumer.NewNotificationEventConsumer(
				normalReader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo,
			).WithDLQWriter(dlqWriter).WithDeviceRepo(deviceRepo)
			if primaryConsumer == nil {
				primaryConsumer = normalConsumer
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				normalConsumer.Start(ctxWorkers)
			}()
			slog.Info("Notification NORMAL-PRIORITY consumer started", slog.String("topic", normalTopic))
		} else {
			slog.Warn("Failed to get kafka reader for normal-priority topic", slog.String("topic", normalTopic), slog.Any("error", normalErr))
		}

		// 3. CORE DOMAIN Inbound Event Consumers (Coaching, Nutrition, Workout Execution)
		coreTopics := []string{
			"coaching.events",
			"nutrition.events",
			"workout_execution.events",
		}
		for _, topic := range coreTopics {
			reader, rErr := deps.KafkaRegistry.GetReader("notification-group-core", topic)
			if rErr == nil && reader != nil {
				consumer := notificationConsumer.NewNotificationEventConsumer(
					reader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo,
				).WithDLQWriter(dlqWriter).WithDeviceRepo(deviceRepo)
				if primaryConsumer == nil {
					primaryConsumer = consumer
				}
				wg.Add(1)
				go func(c *notificationConsumer.NotificationEventConsumer) {
					defer wg.Done()
					c.Start(ctxWorkers)
				}(consumer)
				slog.Info("Notification Core consumer started", slog.String("topic", topic))
			} else {
				slog.Warn("Failed to get kafka reader for core topic", slog.String("topic", topic), slog.Any("error", rErr))
			}
		}

		// Inbound Event Failure Retry Worker: Dispatches through full multi-channel ProcessMessage
		if primaryConsumer == nil {
			primaryConsumer = notificationConsumer.NewNotificationEventConsumer(
				nil, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo,
			).WithDLQWriter(dlqWriter).WithDeviceRepo(deviceRepo)
		}
		inboundRetryWorker := notificationWorker.NewInboundEventRetryWorker(outboxLogRepo, primaryConsumer, 10*time.Second)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = inboundRetryWorker.Start(ctxWorkers)
		}()
	}

	cleanup := func() {
		if cancelWorkers != nil {
			cancelWorkers()
		}
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			slog.Info("All notification background workers stopped gracefully")
		case <-time.After(10 * time.Second):
			slog.Warn("Notification background workers shutdown timed out after 10s")
		}

		if kafkaPub != nil {
			_ = kafkaPub.Close()
		}
		slog.Info("Notification module cleaned up successfully")
	}

	slog.Info("Initialized isolated Notification Bounded Context successfully")
	return grpcHandler, cleanup, nil
}

// RegisterConnectHandler registers the ConnectRPC handler for the Notification module on a connect.Server.
func RegisterConnectHandler(
	server *connect.Server,
	grpcHandler *notificationGRPC.GRPCHandler,
) string {
	connectHandler := notificationGRPC.NewConnectNotificationHandler(grpcHandler)
	notificationv1serviceconnect.RegisterNotificationServiceHandler(server, connectHandler)
	return "/" + notificationv1serviceconnect.NotificationServiceName + "/"
}
