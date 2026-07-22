package domain

import "errors"

var (
	ErrInvalidRoadmap          = errors.New("invalid workout roadmap")
	ErrInvalidSchedule         = errors.New("invalid weekly schedule")
	ErrInvalidPlan             = errors.New("invalid daily workout plan")
	ErrInvalidPrescription     = errors.New("invalid workout prescription")
	ErrOverloadHardCapExceeded = errors.New("overload hard cap exceeded")
	ErrMuscleRecoveryViolation = errors.New("muscle recovery rule violated")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)
