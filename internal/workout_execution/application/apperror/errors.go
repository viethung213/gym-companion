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
	ErrMatchRoomFull              = errors.New("match room is full: maximum 2 participants allowed")
	ErrMatchExerciseMismatch      = errors.New("exercise does not match the solo match room exercise")
)
