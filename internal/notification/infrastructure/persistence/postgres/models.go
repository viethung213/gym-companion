package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

type DeviceModel struct {
	ID          string    `db:"id"`
	UserID      string    `db:"user_id"`
	DeviceToken string    `db:"device_token"`
	DeviceType  string    `db:"device_type"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	LastUsedAt  time.Time `db:"last_used_at"`
}

func (m *DeviceModel) ToDomain() (*aggregate.Device, error) {
	dt, err := vo.NewDeviceType(m.DeviceType)
	if err != nil {
		return nil, fmt.Errorf("invalid device type '%s' in database: %w", m.DeviceType, err)
	}
	return aggregate.ReconstituteDevice(
		m.ID,
		m.UserID,
		m.DeviceToken,
		dt,
		m.IsActive,
		m.CreatedAt,
		m.UpdatedAt,
		m.LastUsedAt,
	), nil
}

type SettingModel struct {
	UserID          string    `db:"user_id"`
	EnablePush      bool      `db:"enable_push"`
	EnableEmail     bool      `db:"enable_email"`
	EnableSMS       bool      `db:"enable_sms"`
	QuietHoursStart string    `db:"quiet_hours_start"`
	QuietHoursEnd   string    `db:"quiet_hours_end"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (m *SettingModel) ToDomain() *aggregate.Setting {
	return aggregate.ReconstituteSetting(
		m.UserID,
		m.EnablePush,
		m.EnableEmail,
		m.EnableSMS,
		m.QuietHoursStart,
		m.QuietHoursEnd,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

type InAppNotificationModel struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Title     string    `db:"title"`
	Body      string    `db:"body"`
	Data      []byte    `db:"data"`
	IsRead    bool      `db:"is_read"`
	CreatedAt time.Time `db:"created_at"`
}

func (m *InAppNotificationModel) ToDomain() (*aggregate.InAppNotification, error) {
	dataMap := make(map[string]string)
	if len(m.Data) > 0 {
		if err := json.Unmarshal(m.Data, &dataMap); err != nil {
			return nil, fmt.Errorf("unmarshal in-app notification data JSON: %w", err)
		}
	}
	return aggregate.ReconstituteInAppNotification(
		m.ID,
		m.UserID,
		m.Title,
		m.Body,
		dataMap,
		m.IsRead,
		m.CreatedAt,
	), nil
}
