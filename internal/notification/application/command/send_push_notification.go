package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"google.golang.org/protobuf/encoding/protojson"
)

type SendPushNotificationCommand struct {
	UserID string
	Title  string
	Body   string
	Data   map[string]string
}

type SendPushNotificationResponse struct {
	NotificationID string
	Status         string
	Message        string
}

type SendPushNotificationHandler struct {
	deviceRepo       repository.DeviceRepository
	notificationRepo repository.NotificationRepository
	settingRepo      repository.SettingRepository
	pushProvider     port.PushProvider
	txManager        port.TxManager
	outboxRepo       port.OutboxRepository
}

func NewSendPushNotificationHandler(
	deviceRepo repository.DeviceRepository,
	notificationRepo repository.NotificationRepository,
	settingRepo repository.SettingRepository,
	pushProvider port.PushProvider,
	txManager port.TxManager,
	outboxRepo port.OutboxRepository,
) *SendPushNotificationHandler {
	return &SendPushNotificationHandler{
		deviceRepo:       deviceRepo,
		notificationRepo: notificationRepo,
		settingRepo:      settingRepo,
		pushProvider:     pushProvider,
		txManager:        txManager,
		outboxRepo:       outboxRepo,
	}
}

func (h *SendPushNotificationHandler) Handle(ctx context.Context, cmd SendPushNotificationCommand) (*SendPushNotificationResponse, error) {
	// 1. Create In-App Notification record
	notifID := uuid.New().String()
	inAppNotif, err := aggregate.NewInAppNotification(notifID, cmd.UserID, cmd.Title, cmd.Body, cmd.Data)
	if err != nil {
		return nil, fmt.Errorf("create in-app notification: %w", err)
	}

	// 2. Save In-App Notification & Outbox CloudEvent 1.0 atomically inside a single DB Transaction
	saveTxFn := func(txCtx context.Context) error {
		if saveErr := h.notificationRepo.Save(txCtx, inAppNotif); saveErr != nil {
			return fmt.Errorf("save in-app notification: %w", saveErr)
		}

		if h.outboxRepo != nil {
			now := time.Now().UTC()
			eventID := uuid.New().String()
			eventType := "contracts.generic.notification.v1.event.NotificationSent"

			payloadProto := &notificationv1event.NotificationSent{
				NotificationId: inAppNotif.ID(),
				UserId:         cmd.UserID,
				Channel:        "PUSH",
				Title:          cmd.Title,
				Body:           cmd.Body,
				SentAt:         now.Format(time.RFC3339),
			}
			payloadBytes, marshalErr := protojson.Marshal(payloadProto)
			if marshalErr != nil {
				return fmt.Errorf("marshal NotificationSent proto payload: %w", marshalErr)
			}

			cloudEvent := map[string]interface{}{
				"specversion":     "1.0",
				"id":              eventID,
				"source":          "services/notification-service",
				"type":            eventType,
				"time":            now.Format(time.RFC3339),
				"datacontenttype": "application/json",
				"data":            json.RawMessage(payloadBytes),
			}

			cloudEventBytes, jsonErr := json.Marshal(cloudEvent)
			if jsonErr != nil {
				return fmt.Errorf("marshal cloudevent envelope: %w", jsonErr)
			}

			outboxRec := &port.OutboxRecord{
				ID:           uuid.New().String(),
				EventID:      eventID,
				EventType:    eventType,
				Payload:      cloudEventBytes,
				PartitionKey: cmd.UserID,
				Published:    false,
				CreatedAt:    now,
			}
			if outboxErr := h.outboxRepo.Save(txCtx, outboxRec); outboxErr != nil {
				return fmt.Errorf("save outbox event: %w", outboxErr)
			}
		}
		return nil
	}

	if h.txManager != nil {
		if txErr := h.txManager.ExecTx(ctx, saveTxFn); txErr != nil {
			return nil, fmt.Errorf("transaction save notification & outbox: %w", txErr)
		}
	} else {
		if execErr := saveTxFn(ctx); execErr != nil {
			return nil, execErr
		}
	}

	// 3. Check User Notification Preferences (enable_push & quiet_hours)
	if h.settingRepo != nil {
		setting, setErr := h.settingRepo.GetByUserID(ctx, cmd.UserID)
		if setErr != nil && errors.Is(setErr, derror.ErrSettingNotFound) {
			var defaultErr error
			setting, defaultErr = aggregate.NewDefaultSetting(cmd.UserID)
			if defaultErr != nil {
				return nil, fmt.Errorf("create default notification setting: %w", defaultErr)
			}
		} else if setErr != nil {
			return nil, fmt.Errorf("get notification settings for user %s: %w", cmd.UserID, setErr)
		}

		if setting != nil {
			if !setting.EnablePush() {
				log.Printf("[Push Suppressed] User %s has disabled push notifications in settings", cmd.UserID)
				return &SendPushNotificationResponse{
					NotificationID: inAppNotif.ID(),
					Status:         "PUSH_DISABLED_BY_USER",
					Message:        "Notification saved to history, but user disabled push notifications in settings",
				}, nil
			}

			if setting.IsInQuietHours(time.Now()) {
				log.Printf("[Push Suppressed] User %s is currently in quiet hours (%s - %s)",
					cmd.UserID, setting.QuietHoursStart(), setting.QuietHoursEnd())
				return &SendPushNotificationResponse{
					NotificationID: inAppNotif.ID(),
					Status:         "QUIET_HOURS_ACTIVE",
					Message:        "Notification saved to history, but push was suppressed during quiet hours",
				}, nil
			}
		}
	}

	// 4. Fetch active device tokens for the user
	devices, err := h.deviceRepo.GetActiveDevicesByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("get active devices for user %s: %w", cmd.UserID, err)
	}

	tokens := make([]string, 0, len(devices))
	for _, dev := range devices {
		tokens = append(tokens, dev.DeviceToken())
	}

	if len(tokens) == 0 {
		return &SendPushNotificationResponse{
			NotificationID: inAppNotif.ID(),
			Status:         "NO_ACTIVE_DEVICES",
			Message:        "Notification saved to history, but user has no active push device tokens",
		}, nil
	}

	// 5. Send via PushProvider (FCM)
	pushResp, err := h.pushProvider.SendPush(ctx, tokens, cmd.Title, cmd.Body, cmd.Data)
	if err != nil {
		return nil, fmt.Errorf("send push via provider: %w", err)
	}

	// 6. Deactivate invalid/unregistered tokens if any returned by FCM
	if len(pushResp.InvalidTokens) > 0 {
		if deactErr := h.deviceRepo.DeactivateTokens(ctx, pushResp.InvalidTokens); deactErr != nil {
			log.Printf("Warning: failed to deactivate invalid tokens: %v", deactErr)
		}
	}

	status := "SENT"
	if pushResp.SuccessCount == 0 && pushResp.FailureCount > 0 {
		status = "FAILED"
	}

	return &SendPushNotificationResponse{
		NotificationID: inAppNotif.ID(),
		Status:         status,
		Message:        fmt.Sprintf("Dispatched to %d tokens (%d success, %d failed)", len(tokens), pushResp.SuccessCount, pushResp.FailureCount),
	}, nil
}
