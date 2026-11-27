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

	var (
		kafkaPub    *notificationKafka.Publisher
		consumerMgr *notificationConsumer.ConsumerManager
	)
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

		// Inbound Event Consumers managed by ConsumerManager
		consumerMgr = notificationConsumer.NewConsumerManager(
			deps.KafkaRegistry,
			sendPushHandler,
			sendEmailHandler,
			sendSMSHandler,
			outboxLogRepo,
			deviceRepo,
		)
		if consumerMgr != nil {
			consumerMgr.Start(ctxWorkers)
		}

		// Inbound Event Failure Retry Worker: Dispatches through full multi-channel ProcessMessage
		var primaryConsumer *notificationConsumer.NotificationEventConsumer
		if consumerMgr != nil {
			primaryConsumer = consumerMgr.PrimaryConsumer()
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
			if consumerMgr != nil {
				consumerMgr.Wait()
			}
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
