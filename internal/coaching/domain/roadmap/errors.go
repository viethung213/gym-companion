package roadmap

import "errors"

var (
	// ErrInvalidRoadmap indicates the roadmap fails structural or business validation.
	ErrInvalidRoadmap = errors.New("invalid roadmap")
	// ErrInvalidStatus indicates an unknown or zero-value status.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrInvalidPhase indicates an unknown or zero-value phase.
	ErrInvalidPhase = errors.New("invalid phase")
	// ErrInvalidTransition indicates an illegal lifecycle transition.
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrWeeklyCapExceeded is returned when adding sessions would exceed BR-AC-01 (6/week).
	ErrWeeklyCapExceeded = errors.New("weekly session cap exceeded (BR-AC-01: max 6/week)")
	// ErrSessionAlreadyFinal is returned when trying to mutate a COMPLETED or SKIPPED session.
	ErrSessionAlreadyFinal = errors.New("session plan already reached final state")
	// ErrSessionNotFound is returned when looking up a session by ID inside a Roadmap.
	ErrSessionNotFound = errors.New("session plan not found")
	// ErrRoadmapNotFound is used by repositories when the aggregate isn't found.
	ErrRoadmapNotFound = errors.New("roadmap not found")
	// ErrActiveRoadmapExists is returned when creating a new roadmap while an ACTIVE one exists.
	ErrActiveRoadmapExists = errors.New("user already has an active roadmap")
	// ErrInvalidWeekCount is returned when the roadmap doesn't contain exactly 4 weeks.
	ErrInvalidWeekCount = errors.New("roadmap must contain exactly 4 week plans")
)
