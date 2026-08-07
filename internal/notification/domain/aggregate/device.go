package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

// Device represents a user's registered device token aggregate.
type Device struct {
	id          string
	userID      string
	deviceToken string
	deviceType  vo.DeviceType
	isActive    bool
	createdAt   time.Time
	updatedAt   time.Time
	lastUsedAt  time.Time
}

func NewDevice(id, userID, deviceToken string, deviceType vo.DeviceType) (*Device, error) {
	if userID == "" {
		return nil, derror.ErrEmptyUserID
	}
	if deviceToken == "" {
		return nil, derror.ErrEmptyDeviceToken
	}

	now := time.Now().UTC()
	return &Device{
		id:          id,
		userID:      userID,
		deviceToken: deviceToken,
		deviceType:  deviceType,
		isActive:    true,
		createdAt:   now,
		updatedAt:   now,
		lastUsedAt:  now,
	}, nil
}

func ReconstituteDevice(
	id, userID, deviceToken string,
	deviceType vo.DeviceType,
	isActive bool,
	createdAt, updatedAt, lastUsedAt time.Time,
) *Device {
	return &Device{
		id:          id,
		userID:      userID,
		deviceToken: deviceToken,
		deviceType:  deviceType,
		isActive:    isActive,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		lastUsedAt:  lastUsedAt,
	}
}

func (d *Device) ID() string                { return d.id }
func (d *Device) UserID() string            { return d.userID }
func (d *Device) DeviceToken() string       { return d.deviceToken }
func (d *Device) DeviceType() vo.DeviceType { return d.deviceType }
func (d *Device) IsActive() bool            { return d.isActive }
func (d *Device) CreatedAt() time.Time      { return d.createdAt }
func (d *Device) UpdatedAt() time.Time      { return d.updatedAt }
func (d *Device) LastUsedAt() time.Time     { return d.lastUsedAt }

func (d *Device) Activate() {
	d.isActive = true
	d.updatedAt = time.Now().UTC()
}

func (d *Device) Deactivate() {
	d.isActive = false
	d.updatedAt = time.Now().UTC()
}

func (d *Device) TouchLastUsed() {
	now := time.Now().UTC()
	d.lastUsedAt = now
	d.updatedAt = now
}
