package command

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type SendSMSNotificationCommand struct {
	UserID         string
	PhoneNumber    string
	Message        string
	IsHighPriority bool
}

type SendSMSNotificationResponse struct {
	Status  string
	Message string
}

type SendSMSNotificationHandler struct {
	settingRepo repository.SettingRepository
	smsProvider port.SMSProvider
}

func NewSendSMSNotificationHandler(
	settingRepo repository.SettingRepository,
	smsProvider port.SMSProvider,
) *SendSMSNotificationHandler {
	return &SendSMSNotificationHandler{
		settingRepo: settingRepo,
		smsProvider: smsProvider,
	}
}

func (h *SendSMSNotificationHandler) Handle(ctx context.Context, cmd SendSMSNotificationCommand) (*SendSMSNotificationResponse, error) {
	if cmd.PhoneNumber == "" {
		return nil, errors.New("recipient phone number is required")
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
			if !setting.EnableSMS() {
				log.Printf("[SMS Suppressed] User %s has disabled SMS notifications in settings", cmd.UserID)
				return &SendSMSNotificationResponse{
					Status:  "SMS_DISABLED_BY_USER",
					Message: "User has disabled SMS notifications in settings",
				}, nil
			}

			if setting.IsInQuietHours(time.Now()) {
				log.Printf("[SMS Suppressed] User %s is currently in quiet hours (%s - %s)",
					cmd.UserID, setting.QuietHoursStart(), setting.QuietHoursEnd())
				return &SendSMSNotificationResponse{
					Status:  "QUIET_HOURS_ACTIVE",
					Message: "SMS suppressed during user quiet hours",
				}, nil
			}
		}
	}

	// 2. Dispatch via SMSProvider (Twilio, Infobip, or Mock adapter)
	if h.smsProvider == nil {
		return &SendSMSNotificationResponse{
			Status:  "NO_PROVIDER",
			Message: "No SMS provider configured",
		}, nil
	}

	if err := h.smsProvider.SendSMS(ctx, cmd.PhoneNumber, cmd.Message); err != nil {
		return nil, fmt.Errorf("send sms via provider: %w", err)
	}

	return &SendSMSNotificationResponse{
		Status:  "SENT",
		Message: "SMS dispatched successfully",
	}, nil
}
