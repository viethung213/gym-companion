package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

	var kafkaPub *notificationKafka.Publisher
	if deps.KafkaRegistry != nil && cfg.KafkaBrokers != "" {
		brokers := strings.Split(cfg.KafkaBrokers, ",")

		// Outbound Outbox Worker Publisher
		writer, wErr := deps.KafkaRegistry.GetWriter("notification.events", brokers)
		if wErr == nil && writer != nil {
			kafkaPub = notificationKafka.NewPublisher(writer)
			outboxWorker := notificationWorker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
			go func() {
				_ = outboxWorker.Start(ctxWorkers)
			}()
		} else {
			slog.Warn("Failed to get kafka writer for notification outbox", slog.Any("error", wErr))
		}

		// 1. HIGH-PRIORITY Notification Consumer: Dedicated group & Multi-worker pool for instant dispatch
		highTopic := "notification.events.high-priority"
		highReader, highErr := deps.KafkaRegistry.GetReader("notification-group-high", highTopic, brokers)
		if highErr == nil && highReader != nil {
			highConsumer := notificationConsumer.NewNotificationEventConsumer(highReader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo)
			const highWorkers = 3
			for i := 0; i < highWorkers; i++ {
				go highConsumer.Start(ctxWorkers)
			}
			slog.Info("Notification HIGH-PRIORITY consumer started", slog.Int("workers", highWorkers), slog.String("topic", highTopic))
		} else {
			slog.Warn("Failed to get kafka reader for high-priority topic", slog.String("topic", highTopic), slog.Any("error", highErr))
		}

		// 2. NORMAL-PRIORITY Notification Consumer: Dedicated group for background/standard dispatch
		normalTopic := "notification.events.normal-priority"
		normalReader, normalErr := deps.KafkaRegistry.GetReader("notification-group-normal", normalTopic, brokers)
		if normalErr == nil && normalReader != nil {
			normalConsumer := notificationConsumer.NewNotificationEventConsumer(normalReader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo)
			go normalConsumer.Start(ctxWorkers)
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
			reader, rErr := deps.KafkaRegistry.GetReader("notification-group-core", topic, brokers)
			if rErr == nil && reader != nil {
				consumer := notificationConsumer.NewNotificationEventConsumer(reader, sendPushHandler, sendEmailHandler, sendSMSHandler, outboxLogRepo)
				go consumer.Start(ctxWorkers)
				slog.Info("Notification Core consumer started", slog.String("topic", topic))
			} else {
				slog.Warn("Failed to get kafka reader for core topic", slog.String("topic", topic), slog.Any("error", rErr))
			}
		}

		// Inbound Event Failure Retry Worker
		inboundRetryWorker := notificationWorker.NewInboundEventRetryWorker(outboxLogRepo, sendPushHandler, 10*time.Second)
		go func() {
			_ = inboundRetryWorker.Start(ctxWorkers)
		}()
	}

	cleanup := func() {
		if cancelWorkers != nil {
			cancelWorkers()
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
