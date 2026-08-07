package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type UpdateNotificationSettingsCommand struct {
	UserID          string
	EnablePush      bool
	EnableEmail     bool
	EnableSMS       bool
	QuietHoursStart string
	QuietHoursEnd   string
}

type UpdateNotificationSettingsHandler struct {
	settingRepo repository.SettingRepository
}

func NewUpdateNotificationSettingsHandler(settingRepo repository.SettingRepository) *UpdateNotificationSettingsHandler {
	return &UpdateNotificationSettingsHandler{settingRepo: settingRepo}
}

func (h *UpdateNotificationSettingsHandler) Handle(ctx context.Context, cmd UpdateNotificationSettingsCommand) error {
	setting, err := h.settingRepo.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		if errors.Is(err, derror.ErrSettingNotFound) {
			newSetting, createErr := aggregate.NewDefaultSetting(cmd.UserID)
			if createErr != nil {
				return fmt.Errorf("create default setting: %w", createErr)
			}
			setting = newSetting
		} else {
			return fmt.Errorf("get notification setting: %w", err)
		}
	}

	if err := setting.Update(cmd.EnablePush, cmd.EnableEmail, cmd.EnableSMS, cmd.QuietHoursStart, cmd.QuietHoursEnd); err != nil {
		return fmt.Errorf("update notification setting: %w", err)
	}

	if err := h.settingRepo.Save(ctx, setting); err != nil {
		return fmt.Errorf("save notification setting: %w", err)
	}

	return nil
}
