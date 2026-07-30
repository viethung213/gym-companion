package derror

import "errors"

// Domain Business Errors for Workout Execution Context
var (
	ErrWorkoutSessionNotFound       = errors.New("workout session not found")
	ErrSessionNotInProgress         = errors.New("workout session is not in progress")
	ErrSessionAlreadyCompleted      = errors.New("workout session is already completed")
	ErrSessionAlreadyAborted        = errors.New("workout session is already aborted")
	ErrActiveSessionAlreadyExists   = errors.New("active workout session already exists for user")
	ErrOverloadConfirmationRequired = errors.New("training load exceeds 250% of recent average; user confirmation required")
	ErrAnomalousSessionTimeout      = errors.New("workout session exceeds 240 minutes without activity and is marked anomalous")
	ErrInvalidROM                   = errors.New("range of motion percentage is invalid")
	ErrMotionSpecNotFound           = errors.New("motion specification not found for exercise")
	ErrPersonalRecordNotFound       = errors.New("personal record not found")
	ErrInvalidSetNumber             = errors.New("invalid set number")
	ErrInvalidRepsOrWeight          = errors.New("invalid reps or weight value")
	ErrNotFound                     = ErrMotionSpecNotFound
)
