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

type SendEmailNotificationCommand struct {
	UserID         string
	To             string
	Subject        string
	Body           string
	IsHighPriority bool
}

type SendEmailNotificationResponse struct {
	Status  string
	Message string
}

type SendEmailNotificationHandler struct {
	settingRepo   repository.SettingRepository
	emailProvider port.EmailProvider
}

func NewSendEmailNotificationHandler(
	settingRepo repository.SettingRepository,
	emailProvider port.EmailProvider,
) *SendEmailNotificationHandler {
	return &SendEmailNotificationHandler{
		settingRepo:   settingRepo,
		emailProvider: emailProvider,
	}
}

func (h *SendEmailNotificationHandler) Handle(ctx context.Context, cmd SendEmailNotificationCommand) (*SendEmailNotificationResponse, error) {
	if cmd.To == "" {
		return nil, errors.New("recipient email (To) is required")
	}

	// 1. Check User Notification Preferences if UserID is provided
	// High priority notifications bypass user settings and quiet hours
	if !cmd.IsHighPriority && cmd.UserID != "" && h.settingRepo != nil {
		setting, err := h.settingRepo.GetByUserID(ctx, cmd.UserID)
		if err != nil && errors.Is(err, derror.ErrSettingNotFound) {
			var defaultErr error
			setting, defaultErr = aggregate.NewDefaultSetting(cmd.UserID)
			if defaultErr != nil {
				return nil, fmt.Errorf("create default notification setting: %w", defaultErr)
			}
		} else if err != nil {
			return nil, fmt.Errorf("get notification settings for user %s: %w", cmd.UserID, err)
		}

		if setting != nil {
			if !setting.EnableEmail() {
				slog.InfoContext(ctx, "User has disabled email notifications in settings", slog.String("user_id", cmd.UserID))
				return &SendEmailNotificationResponse{
					Status:  "EMAIL_DISABLED_BY_USER",
					Message: "User has disabled email notifications in settings",
				}, nil
			}

			if setting.IsInQuietHours(time.Now()) {
				slog.InfoContext(ctx, "User is currently in quiet hours, email suppressed",
					slog.String("user_id", cmd.UserID),
					slog.String("quiet_start", setting.QuietHoursStart()),
					slog.String("quiet_end", setting.QuietHoursEnd()))
				return &SendEmailNotificationResponse{
					Status:  "QUIET_HOURS_ACTIVE",
					Message: "Email suppressed during user quiet hours",
				}, nil
			}
		}
	}

	// 2. Dispatch via EmailProvider (Gmail SMTP or configured adapter)
	if h.emailProvider == nil {
		return &SendEmailNotificationResponse{
			Status:  "NO_PROVIDER",
			Message: "No email provider configured",
		}, nil
	}

	if err := h.emailProvider.SendEmail(ctx, cmd.To, cmd.Subject, cmd.Body); err != nil {
		return nil, fmt.Errorf("send email via provider: %w", err)
	}

	return &SendEmailNotificationResponse{
		Status:  "SENT",
		Message: "Email dispatched successfully",
	}, nil
}
