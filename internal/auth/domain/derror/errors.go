package derror

import "errors"

// Domain Business Errors
var (
	ErrNotFound              = errors.New("entity not found")
	ErrConflict              = errors.New("resource conflict")
	ErrUnauthorized          = errors.New("unauthorized access")
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidEmail          = errors.New("invalid email format")
	ErrInvalidPhone          = errors.New("invalid phone number format")
	ErrInvalidRole           = errors.New("invalid user role")
	ErrIdentityAlreadyExists = errors.New("identity already exists")
	ErrIdentityNotFound      = errors.New("identity not found")
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters long")
	ErrPasswordMismatch      = errors.New("new password and confirmation do not match")
	ErrEmptyHashedPassword   = errors.New("hashed password cannot be empty")

	// OTP Domain Errors
	ErrOTPAlreadyUsed         = errors.New("otp code has already been used")
	ErrOTPExpired             = errors.New("otp code has expired")
	ErrOTPMaxAttemptsExceeded = errors.New("maximum otp verification attempts exceeded")
	ErrOTPInvalidCode         = errors.New("invalid otp verification code")
	ErrOTPResendCooldown      = errors.New("please wait before requesting another otp code")

	// Password Reset Token Errors
	ErrResetTokenAlreadyUsed = errors.New("password reset token has already been used")
	ErrResetTokenExpired     = errors.New("password reset token has expired")
)
