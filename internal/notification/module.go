package notification

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/service/notificationv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/fcm"
	notificationKafka "github.com/viethung213/gym-companion/internal/notification/infrastructure/kafka"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/persistence/postgres"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/transport"
	notificationWorker "github.com/viethung213/gym-companion/internal/notification/infrastructure/worker"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type ModuleDeps struct {
	DB            *sql.DB
	KafkaRegistry *sharedKafka.Registry
}

func Initialize(ctx context.Context, deps ModuleDeps) (*transport.GRPCHandler, func(), error) {
	if deps.DB == nil {
		return nil, nil, errors.New("deps.DB is required")
	}

	cfg := config.LoadConfig()

	// 1. Repositories & Adapters
	deviceRepo := postgres.NewDeviceRepository(deps.DB)
	settingRepo := postgres.NewSettingRepository(deps.DB)
	notificationRepo := postgres.NewNotificationRepository(deps.DB)
	outboxRepo := postgres.NewOutboxRepository(deps.DB)
	outboxLogRepo := postgres.NewOutboxLogRepository(deps.DB)
	txManager := postgres.NewTxManager(deps.DB)
	fcmClient := fcm.NewClient(cfg)

	// 2. Command Handlers
	registerDeviceHandler := command.NewRegisterDeviceTokenHandler(deviceRepo)
	sendPushHandler := command.NewSendPushNotificationHandler(deviceRepo, notificationRepo, settingRepo, fcmClient, txManager, outboxRepo)
	updateSettingsHandler := command.NewUpdateNotificationSettingsHandler(settingRepo)
	markAsReadHandler := command.NewMarkNotificationAsReadHandler(notificationRepo)

	// 3. Query Handlers
	getSettingsHandler := query.NewGetNotificationSettingsHandler(settingRepo)
	listNotifsHandler := query.NewListNotificationsHandler(notificationRepo)

	// 4. gRPC Transport Handler
	grpcHandler := transport.NewGRPCHandler(
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
		brokers := []string{cfg.KafkaBrokers}

		// Outbound Outbox Worker Publisher
		writer, wErr := deps.KafkaRegistry.GetWriter("events.v1", brokers)
		if wErr == nil && writer != nil {
			kafkaPub = notificationKafka.NewPublisher(writer)
			outboxWorker := notificationWorker.NewOutboxWorker(outboxRepo, outboxLogRepo, kafkaPub, 2*time.Second)
			go func() {
				_ = outboxWorker.Start(ctxWorkers)
			}()
		} else {
			log.Printf("Warning: failed to get kafka writer for notification outbox: %v", wErr)
		}

		// Inbound Event Consumer
		reader, rErr := deps.KafkaRegistry.GetReader("notification-group", "events.v1", brokers)
		if rErr == nil && reader != nil {
			consumer := notificationKafka.NewNotificationEventConsumer(reader, sendPushHandler, outboxLogRepo)
			go consumer.Start(ctxWorkers)
		} else {
			log.Printf("Warning: failed to get kafka reader for notification events: %v", rErr)
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
		log.Println("Notification module cleaned up successfully.")
	}

	log.Println("Initialized isolated Notification Bounded Context successfully.")
	return grpcHandler, cleanup, nil
}

// RegisterConnectHandler mounts the ConnectRPC handler for the Notification module on an http.ServeMux.
func RegisterConnectHandler(
	mux *http.ServeMux,
	grpcHandler *transport.GRPCHandler,
	opts ...connect.HandlerOption,
) {
	connectHandler := transport.NewConnectNotificationHandler(grpcHandler)
	path, handler := notificationv1serviceconnect.NewNotificationServiceHandler(connectHandler, opts...)
	mux.Handle(path, handler)
}
