package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ repository.DeviceRepository = (*DeviceRepository)(nil)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Save(ctx context.Context, device *aggregate.Device) error {
	model := ToDeviceModel(device)
	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "device_token"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"is_active",
			"device_type",
			"updated_at",
			"last_used_at",
		}),
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("upsert user device token: %w", err)
	}
	return nil
}

func (r *DeviceRepository) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	var models []DeviceModel
	err := GetDB(ctx, r.db).
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("last_used_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("query active devices: %w", err)
	}

	devices := make([]*aggregate.Device, 0, len(models))
	for i := range models {
		dev, err := models[i].ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert device model to domain: %w", err)
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

func (r *DeviceRepository) DeactivateTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}

	err := GetDB(ctx, r.db).Model(&DeviceModel{}).
		Where("device_token IN ?", tokens).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		}).Error
	if err != nil {
		return fmt.Errorf("deactivate device tokens: %w", err)
	}
	return nil
}
