package apperror

import "errors"

var (
	ErrNotFound        = errors.New("requested resource not found")
	ErrInvalidArgument = errors.New("invalid arguments provided")
	ErrConflict        = errors.New("resource state conflict")
	ErrInternal        = errors.New("internal application error")
)
