package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

type DeviceModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	UserID      string    `gorm:"column:user_id;index:idx_notification_user_devices_user_id"`
	DeviceToken string    `gorm:"column:device_token"`
	DeviceType  string    `gorm:"column:device_type"`
	IsActive    bool      `gorm:"column:is_active;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	LastUsedAt  time.Time `gorm:"column:last_used_at"`
}

func (DeviceModel) TableName() string {
	return "notification.user_devices"
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

func ToDeviceModel(d *aggregate.Device) *DeviceModel {
	return &DeviceModel{
		ID:          d.ID(),
		UserID:      d.UserID(),
		DeviceToken: d.DeviceToken(),
		DeviceType:  d.DeviceType().String(),
		IsActive:    d.IsActive(),
		CreatedAt:   d.CreatedAt(),
		UpdatedAt:   d.UpdatedAt(),
		LastUsedAt:  d.LastUsedAt(),
	}
}

type SettingModel struct {
	UserID          string    `gorm:"primaryKey;column:user_id"`
	EnablePush      bool      `gorm:"column:enable_push;default:true"`
	EnableEmail     bool      `gorm:"column:enable_email;default:true"`
	EnableSMS       bool      `gorm:"column:enable_sms;default:true"`
	QuietHoursStart string    `gorm:"column:quiet_hours_start;default:''"`
	QuietHoursEnd   string    `gorm:"column:quiet_hours_end;default:''"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (SettingModel) TableName() string {
	return "notification.user_settings"
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

func ToSettingModel(s *aggregate.Setting) *SettingModel {
	return &SettingModel{
		UserID:          s.UserID(),
		EnablePush:      s.EnablePush(),
		EnableEmail:     s.EnableEmail(),
		EnableSMS:       s.EnableSMS(),
		QuietHoursStart: s.QuietHoursStart(),
		QuietHoursEnd:   s.QuietHoursEnd(),
		CreatedAt:       s.CreatedAt(),
		UpdatedAt:       s.UpdatedAt(),
	}
}

type InAppNotificationModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	UserID    string    `gorm:"column:user_id;index:idx_notification_in_app_notifications_user_id"`
	Title     string    `gorm:"column:title"`
	Body      string    `gorm:"column:body"`
	Data      []byte    `gorm:"column:data;type:jsonb"`
	IsRead    bool      `gorm:"column:is_read;default:false"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (InAppNotificationModel) TableName() string {
	return "notification.in_app_notifications"
}

func (m *InAppNotificationModel) ToDomain() (*aggregate.InAppNotification, error) {
	dataMap := make(map[string]string)
	if len(m.Data) > 0 {
		var rawMap map[string]any
		if err := json.Unmarshal(m.Data, &rawMap); err != nil {
			if errDirect := json.Unmarshal(m.Data, &dataMap); errDirect != nil {
				return nil, fmt.Errorf("unmarshal in-app notification data JSON: %w", err)
			}
		} else {
			for k, v := range rawMap {
				dataMap[k] = fmt.Sprintf("%v", v)
			}
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

func ToInAppNotificationModel(n *aggregate.InAppNotification) (*InAppNotificationModel, error) {
	dataBytes, err := json.Marshal(n.Data())
	if err != nil {
		return nil, fmt.Errorf("marshal notification data JSON: %w", err)
	}
	return &InAppNotificationModel{
		ID:        n.ID(),
		UserID:    n.UserID(),
		Title:     n.Title(),
		Body:      n.Body(),
		Data:      dataBytes,
		IsRead:    n.IsRead(),
		CreatedAt: n.CreatedAt(),
	}, nil
}

type OutboxModel struct {
	ID            string     `gorm:"primaryKey;column:id"`
	EventID       string     `gorm:"column:event_id;uniqueIndex"`
	AggregateType string     `gorm:"column:aggregate_type"`
	AggregateID   string     `gorm:"column:aggregate_id"`
	EventType     string     `gorm:"column:event_type"`
	Payload       []byte     `gorm:"column:payload;type:jsonb"`
	PartitionKey  string     `gorm:"column:partition_key"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	Published     bool       `gorm:"column:published"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	Status        string     `gorm:"column:status"`
	LockedUntil   *time.Time `gorm:"column:locked_until"`
}

func (OutboxModel) TableName() string {
	return "notification.outbox"
}

func (m *OutboxModel) ToDomain() *port.OutboxRecord {
	return &port.OutboxRecord{
		ID:            m.ID,
		EventID:       m.EventID,
		AggregateType: m.AggregateType,
		AggregateID:   m.AggregateID,
		EventType:     m.EventType,
		Payload:       m.Payload,
		PartitionKey:  m.PartitionKey,
		CreatedAt:     m.CreatedAt,
		Published:     m.Published,
		PublishedAt:   m.PublishedAt,
	}
}

type OutboxLogModel struct {
	ID           string    `gorm:"primaryKey;column:id"`
	EventID      string    `gorm:"column:event_id"`
	EventType    string    `gorm:"column:event_type"`
	Payload      []byte    `gorm:"column:payload;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (OutboxLogModel) TableName() string {
	return "notification.outbox_log"
}

var _ = sql.ErrNoRows
