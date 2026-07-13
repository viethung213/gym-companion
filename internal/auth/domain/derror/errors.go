package derror

import "errors"

// Domain Business Errors
var (
	ErrUserNotFound  = errors.New("user not found")
	ErrInvalidEmail  = errors.New("invalid email format")
	ErrInvalidRole   = errors.New("invalid user role")
	ErrInvalidUserID = errors.New("invalid user id format")
)
