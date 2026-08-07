package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
)

type DeviceRepository interface {
	Save(ctx context.Context, device *aggregate.Device) error
	GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error)
	DeactivateTokens(ctx context.Context, tokens []string) error
}
