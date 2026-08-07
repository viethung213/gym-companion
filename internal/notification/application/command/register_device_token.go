package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

type RegisterDeviceTokenCommand struct {
	UserID      string
	DeviceToken string
	DeviceType  string
}

type RegisterDeviceTokenHandler struct {
	deviceRepo repository.DeviceRepository
}

func NewRegisterDeviceTokenHandler(deviceRepo repository.DeviceRepository) *RegisterDeviceTokenHandler {
	return &RegisterDeviceTokenHandler{deviceRepo: deviceRepo}
}

func (h *RegisterDeviceTokenHandler) Handle(ctx context.Context, cmd RegisterDeviceTokenCommand) error {
	dt, err := vo.NewDeviceType(cmd.DeviceType)
	if err != nil {
		return fmt.Errorf("register device token type: %w", err)
	}

	id := uuid.New().String()
	device, err := aggregate.NewDevice(id, cmd.UserID, cmd.DeviceToken, dt)
	if err != nil {
		return fmt.Errorf("create device aggregate: %w", err)
	}

	if err := h.deviceRepo.Save(ctx, device); err != nil {
		return fmt.Errorf("save device token: %w", err)
	}

	return nil
}
