package apperror

import "errors"

var (
	ErrInvalidRequest = errors.New("invalid request arguments")
	ErrInternalServer = errors.New("internal server error")
)
