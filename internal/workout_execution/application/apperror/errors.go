package apperror

import "errors"

// Application Layer Errors for Workout Execution Context
var (
	ErrInvalidInput               = errors.New("invalid request parameters")
	ErrDailyPlanNotFound          = errors.New("daily workout plan not found")
	ErrExerciseCatalogUnavailable = errors.New("exercise catalog service unavailable")
	ErrUserProfileUnavailable     = errors.New("user profile service unavailable")
	ErrEventPublishFailed         = errors.New("failed to publish outbox event")
	ErrTransactionFailed          = errors.New("database transaction failed")
	ErrConflict                   = errors.New("resource conflict")
	ErrInternal                   = errors.New("internal server error")
)
