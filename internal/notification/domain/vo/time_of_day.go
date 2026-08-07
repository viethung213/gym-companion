package vo

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidTimeFormat = errors.New("time of day must be in HH:MM 24-hour format (00:00 - 23:59) or empty")

// TimeOfDay is a Value Object that encapsulates 24-hour time of day (HH:MM).
type TimeOfDay struct {
	value string
}

func NewTimeOfDay(raw string) (TimeOfDay, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TimeOfDay{value: ""}, nil
	}

	if len(trimmed) != 5 {
		return TimeOfDay{}, ErrInvalidTimeFormat
	}

	_, err := time.Parse("15:04", trimmed)
	if err != nil {
		return TimeOfDay{}, ErrInvalidTimeFormat
	}

	return TimeOfDay{value: trimmed}, nil
}

func (t TimeOfDay) String() string {
	return t.value
}

func (t TimeOfDay) IsEmpty() bool {
	return t.value == ""
}
