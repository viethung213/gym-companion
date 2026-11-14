package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	domainEvent "github.com/viethung213/gym-companion/internal/notification/domain/event"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type SendPushNotificationCommand struct {
	UserID         string
	Title          string
	Body           string
	Data           map[string]string
	IsHighPriority bool
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
	eventRecorder    port.EventRecorder
}

func NewSendPushNotificationHandler(
	deviceRepo repository.DeviceRepository,
	notificationRepo repository.NotificationRepository,
	settingRepo repository.SettingRepository,
	pushProvider port.PushProvider,
	txManager port.TxManager,
	eventRecorder port.EventRecorder,
) *SendPushNotificationHandler {
	return &SendPushNotificationHandler{
		deviceRepo:       deviceRepo,
		notificationRepo: notificationRepo,
		settingRepo:      settingRepo,
		pushProvider:     pushProvider,
		txManager:        txManager,
		eventRecorder:    eventRecorder,
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

		if h.eventRecorder != nil {
			ev := domainEvent.NewNotificationSentEvent(
				inAppNotif.ID(),
				cmd.UserID,
				"PUSH",
				cmd.Title,
				cmd.Body,
				cmd.Data,
			)
			if outboxErr := h.eventRecorder.RecordEvents(txCtx, []any{ev}); outboxErr != nil {
				return fmt.Errorf("record notification outbox event: %w", outboxErr)
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
	// High priority notifications bypass user settings and quiet hours
	if !cmd.IsHighPriority && h.settingRepo != nil {
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
				slog.InfoContext(ctx, "User has disabled push notifications in settings", slog.String("user_id", cmd.UserID))
				return &SendPushNotificationResponse{
					NotificationID: inAppNotif.ID(),
					Status:         "PUSH_DISABLED_BY_USER",
					Message:        "Notification saved to history, but user disabled push notifications in settings",
				}, nil
			}

			if setting.IsInQuietHours(time.Now()) {
				slog.InfoContext(ctx, "User is currently in quiet hours, push suppressed",
					slog.String("user_id", cmd.UserID),
					slog.String("quiet_start", setting.QuietHoursStart()),
					slog.String("quiet_end", setting.QuietHoursEnd()))
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
			slog.WarnContext(ctx, "Warning: failed to deactivate invalid tokens", slog.Any("error", deactErr))
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
