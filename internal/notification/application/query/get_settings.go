package query

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type GetNotificationSettingsQuery struct {
	UserID string
}

type GetNotificationSettingsHandler struct {
	settingRepo repository.SettingRepository
}

func NewGetNotificationSettingsHandler(settingRepo repository.SettingRepository) *GetNotificationSettingsHandler {
	return &GetNotificationSettingsHandler{settingRepo: settingRepo}
}

func (h *GetNotificationSettingsHandler) Handle(ctx context.Context, q GetNotificationSettingsQuery) (*aggregate.Setting, error) {
	setting, err := h.settingRepo.GetByUserID(ctx, q.UserID)
	if err != nil {
		if errors.Is(err, derror.ErrSettingNotFound) {
			defaultSetting, createErr := aggregate.NewDefaultSetting(q.UserID)
			if createErr != nil {
				return nil, fmt.Errorf("create default setting: %w", createErr)
			}
			return defaultSetting, nil
		}
		return nil, fmt.Errorf("get notification setting: %w", err)
	}

	return setting, nil
}
