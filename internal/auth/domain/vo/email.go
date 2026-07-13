package vo

import (
	"regexp"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Email represents a validated email address Value Object.
type Email struct {
	value string
}

// NewEmail parses, sanitizes, and validates an email string.
func NewEmail(v string) (Email, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	if !emailRegex.MatchString(v) {
		return Email{}, derror.ErrInvalidEmail
	}
	return Email{value: v}, nil
}

// Value returns the raw string value of the Email.
func (e Email) Value() string {
	return e.value
}

// IsZero checks if the Email Value Object is empty/uninitialized.
func (e Email) IsZero() bool {
	return e.value == ""
}
