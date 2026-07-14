package vo

import (
	"regexp"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// UserID represents a validated User ID Value Object.
type UserID struct {
	value string
}

// NewUserID parses and validates a user ID string.
func NewUserID(v string) (UserID, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return UserID{}, derror.ErrInvalidUserID
	}
	if !uuidRegex.MatchString(v) {
		return UserID{}, derror.ErrInvalidUserID
	}
	return UserID{value: strings.ToLower(v)}, nil
}

// Value returns the raw string value of the UserID.
func (u UserID) Value() string {
	return u.value
}

// IsZero checks if the UserID Value Object is empty/uninitialized.
func (u UserID) IsZero() bool {
	return u.value == ""
}
