package vo

import (
	"regexp"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

// e164Regex validates standard international E.164 phone format (+ followed by 7 to 15 digits).
var e164Regex = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// Phone represents a validated and normalized E.164 phone number Value Object.
type Phone struct {
	value string
}

// NewPhone parses, normalizes, and validates a phone number.
// It automatically converts local Vietnamese formats (e.g., 0901234567) to E.164 (+84901234567).
func NewPhone(v string) (Phone, error) {
	// Sanitize common separators
	cleaned := strings.TrimSpace(v)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")

	// Normalize Vietnam domestic format: starts with 0 and length is 10 digits
	if strings.HasPrefix(cleaned, "0") && len(cleaned) == 10 {
		cleaned = "+84" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "84") && len(cleaned) == 11 {
		cleaned = "+" + cleaned
	}

	if !e164Regex.MatchString(cleaned) {
		return Phone{}, derror.ErrInvalidPhone
	}

	return Phone{value: cleaned}, nil
}

// Value returns the normalized E.164 phone string.
func (p Phone) Value() string {
	return p.value
}

// IsZero checks if the Phone Value Object is empty/uninitialized.
func (p Phone) IsZero() bool {
	return p.value == ""
}
