package vo

import (
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
)

// Timezone is the athlete's IANA time zone name. Workout slot times and
// scheduled dates are wall clock in this zone.
type Timezone string

// DefaultTimezone applies when the client sent no zone.
const DefaultTimezone Timezone = "UTC"

// maxTimezoneLen bounds the stored name (profile.users.timezone is VARCHAR(64)).
const maxTimezoneLen = 64

// ParseTimezone validates an IANA zone name such as "Asia/Ho_Chi_Minh".
// An empty name yields DefaultTimezone.
func ParseTimezone(name string) (Timezone, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultTimezone, nil
	}
	if len(name) > maxTimezoneLen || strings.EqualFold(name, "Local") {
		return "", fmt.Errorf("%w: %q", derror.ErrInvalidTimezone, name)
	}
	if _, err := time.LoadLocation(name); err != nil {
		return "", fmt.Errorf("%w: %q", derror.ErrInvalidTimezone, name)
	}
	return Timezone(name), nil
}

// String returns the zone name, DefaultTimezone when unset.
func (t Timezone) String() string {
	if t == "" {
		return string(DefaultTimezone)
	}
	return string(t)
}
