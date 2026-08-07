package derror

import "errors"

var (
	ErrDeviceNotFound       = errors.New("device token not found")
	ErrSettingNotFound      = errors.New("notification setting not found")
	ErrNotificationNotFound = errors.New("notification item not found")
	ErrEmptyUserID          = errors.New("user ID cannot be empty")
	ErrEmptyDeviceToken     = errors.New("device token cannot be empty")
	ErrEmptyTitle           = errors.New("notification title cannot be empty")
	ErrInvalidTimeFormat    = errors.New("quiet hours must be in HH:MM 24-hour format or empty")
)
