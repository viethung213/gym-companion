package derror

import "errors"

var (
	ErrProfileNotFound     = errors.New("user profile not found")
	ErrInvalidBiological   = errors.New("invalid biological metrics: weight, height, or age out of range")
	ErrInjuryNotFound      = errors.New("injury not found")
	ErrInjuryAlreadyActive = errors.New("injury is already active for this muscle group")
	ErrInjuryAlreadyClosed = errors.New("injury has already been marked as recovered")
	ErrInvalidMetric       = errors.New("invalid metric values")
	ErrEmptyIdentityUpdate = errors.New("at least one of full_name or avatar_url must be provided")
	ErrInvalidFullName     = errors.New("full name cannot exceed 100 characters")
)
