package vo

import (
	"errors"
	"strings"
)

// ErrInvalidDeviceType represents an error when an unsupported device type is provided.
var ErrInvalidDeviceType = errors.New("invalid device type: must be IOS, ANDROID, or WEB")

// DeviceType represents the platform type of user device.
type DeviceType string

const (
	DeviceTypeIOS     DeviceType = "IOS"
	DeviceTypeAndroid DeviceType = "ANDROID"
	DeviceTypeWeb     DeviceType = "WEB"
)

// NewDeviceType parses and validates a device type string.
func NewDeviceType(s string) (DeviceType, error) {
	upper := DeviceType(strings.ToUpper(strings.TrimSpace(s)))
	switch upper {
	case DeviceTypeIOS, DeviceTypeAndroid, DeviceTypeWeb:
		return upper, nil
	default:
		return "", ErrInvalidDeviceType
	}
}

func (d DeviceType) String() string {
	return string(d)
}
