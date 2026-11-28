package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

// EmailChannelDispatcher handles email delivery across Direct targets.
type EmailChannelDispatcher struct {
	emailProvider port.EmailProvider
	settingRepo   repository.SettingRepository
}

func NewEmailChannelDispatcher(
	emailProvider port.EmailProvider,
	settingRepo repository.SettingRepository,
) *EmailChannelDispatcher {
	return &EmailChannelDispatcher{
		emailProvider: emailProvider,
		settingRepo:   settingRepo,
	}
}

func (d *EmailChannelDispatcher) Channel() string {
	return "EMAIL"
}

func (d *EmailChannelDispatcher) Dispatch(
	ctx context.Context,
	target port.NotificationTarget,
	msg NotificationMessage,
) (*NotificationMessage, error) {
	// Guard clause: Email dispatch is strictly restricted to individual recipients (ScopeDirect).
	// Broadcast and Group scopes are intentionally skipped to protect domain reputation and financial safety.
	if target.Scope != port.ScopeDirect {
		slog.WarnContext(ctx, "Email dispatch skipped: channel is strictly restricted to ScopeDirect",
			slog.String("scope", string(target.Scope)),
			slog.String("event_id", msg.EventID),
		)
		return nil, nil
	}

	emailAddr := extractEmailFromData(msg.Data)
	if emailAddr == "" {
		return nil, nil
	}

	if d.emailProvider == nil {
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

		if setting != nil && !setting.EnableEmail() {
			slog.InfoContext(ctx, "Email notifications disabled by user", slog.String("user_id", target.UserID))
			return nil, nil
		}
	}

	if err := d.emailProvider.SendEmail(ctx, emailAddr, msg.Title, msg.Body); err != nil {
		return nil, err
	}
	return nil, nil
}

func extractEmailFromData(data map[string]string) string {
	if data == nil {
		return ""
	}
	if email := data["email"]; email != "" {
		return email
	}
	if email := data["recipient_email"]; email != "" {
		return email
	}
	return data["to_email"]
}
