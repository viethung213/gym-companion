package apperror

import "errors"

// Application Errors (Session & Key related errors)
var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrSessionNotFound = errors.New("session not found")
	ErrUnauthorized    = errors.New("unauthorized session access")
)
