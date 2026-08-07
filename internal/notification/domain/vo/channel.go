package vo

import (
	"errors"
	"strings"
)

// ErrInvalidChannel represents an error when an unsupported channel type is provided.
var ErrInvalidChannel = errors.New("invalid channel type: must be PUSH, EMAIL, or SMS")

// ChannelType represents the notification delivery channel.
type ChannelType string

const (
	ChannelTypePush  ChannelType = "PUSH"
	ChannelTypeEmail ChannelType = "EMAIL"
	ChannelTypeSMS   ChannelType = "SMS"
)

// NewChannelType parses and validates a channel string.
func NewChannelType(s string) (ChannelType, error) {
	upper := ChannelType(strings.ToUpper(strings.TrimSpace(s)))
	switch upper {
	case ChannelTypePush, ChannelTypeEmail, ChannelTypeSMS:
		return upper, nil
	default:
		return "", ErrInvalidChannel
	}
}

func (c ChannelType) String() string {
	return string(c)
}
