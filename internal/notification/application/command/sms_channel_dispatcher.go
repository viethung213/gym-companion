package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

// SMSChannelDispatcher handles SMS text message delivery directly via SMSProvider and SettingRepository.
type SMSChannelDispatcher struct {
	smsProvider port.SMSProvider
	settingRepo repository.SettingRepository
}

func NewSMSChannelDispatcher(
	smsProvider port.SMSProvider,
	settingRepo repository.SettingRepository,
) *SMSChannelDispatcher {
	return &SMSChannelDispatcher{
		smsProvider: smsProvider,
		settingRepo: settingRepo,
	}
}

func (d *SMSChannelDispatcher) Channel() string {
	return "SMS"
}

func (d *SMSChannelDispatcher) Dispatch(
	ctx context.Context,
	target port.NotificationTarget,
	msg NotificationMessage,
) (*NotificationMessage, error) {
	// Guard clause: SMS dispatch is strictly restricted to individual recipients (ScopeDirect).
	// Broadcast and Group scopes are intentionally skipped to prevent unexpected telecommunication costs.
	if target.Scope != port.ScopeDirect {
		slog.WarnContext(ctx, "SMS dispatch skipped: channel is strictly restricted to ScopeDirect",
			slog.String("scope", string(target.Scope)),
			slog.String("event_id", msg.EventID),
		)
		return nil, nil
	}

	phoneNum := extractPhoneFromData(msg.Data)
	if phoneNum == "" {
		return nil, nil
	}

	if d.smsProvider == nil {
		return nil, nil
	}

	if target.UserID != "" && !msg.IsHighPriority && d.settingRepo != nil {
		setting, setErr := d.settingRepo.GetByUserID(ctx, target.UserID)
		if setErr != nil && errors.Is(setErr, derror.ErrSettingNotFound) {
			var defaultErr error
			setting, defaultErr = aggregate.NewDefaultSetting(target.UserID)
			if defaultErr != nil {
				return nil, fmt.Errorf("create default setting: %w", defaultErr)
			}
		} else if setErr != nil {
			return nil, fmt.Errorf("get notification settings: %w", setErr)
		}

		if setting != nil {
			if !setting.EnableSMS() {
				slog.InfoContext(ctx, "SMS notifications disabled by user", slog.String("user_id", target.UserID))
				return nil, nil
			}

			if setting.IsInQuietHours(time.Now()) {
				slog.InfoContext(ctx, "SMS suppressed during user quiet hours", slog.String("user_id", target.UserID))
				return nil, nil
			}
		}
	}

	if err := d.smsProvider.SendSMS(ctx, phoneNum, msg.Body); err != nil {
		return nil, err
	}
	return nil, nil
}

func extractPhoneFromData(data map[string]string) string {
	if data == nil {
		return ""
	}
	if phone := data["phone"]; phone != "" {
		return phone
	}
	if phone := data["phone_number"]; phone != "" {
		return phone
	}
	if phone := data["recipient_phone"]; phone != "" {
		return phone
	}
	return data["to_phone"]
}
