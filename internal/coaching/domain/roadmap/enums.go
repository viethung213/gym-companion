// Package roadmap contains the Roadmap aggregate and its child entities
// (WeekPlan, DayPlan, SessionPlan) with 4-tier DDD structure.
package roadmap

// RoadmapStatus is the lifecycle status of a Roadmap aggregate.
type RoadmapStatus string

const (
	RoadmapStatusActive    RoadmapStatus = "ACTIVE"
	RoadmapStatusCompleted RoadmapStatus = "COMPLETED"
)

// Valid reports whether s is a known RoadmapStatus.
func (s RoadmapStatus) Valid() bool {
	switch s {
	case RoadmapStatusActive, RoadmapStatusCompleted:
		return true
	default:
		return false
	}
}

// SessionPlanStatus is the lifecycle status of a SessionPlan entity.
type SessionPlanStatus string

const (
	SessionPlanStatusPending   SessionPlanStatus = "PENDING"
	SessionPlanStatusCompleted SessionPlanStatus = "COMPLETED"
	SessionPlanStatusSkipped   SessionPlanStatus = "SKIPPED"
)

// Valid reports whether s is a known SessionPlanStatus.
func (s SessionPlanStatus) Valid() bool {
	switch s {
	case SessionPlanStatusPending, SessionPlanStatusCompleted, SessionPlanStatusSkipped:
		return true
	default:
		return false
	}
}

// IsFinal reports whether the session has reached a terminal state.
func (s SessionPlanStatus) IsFinal() bool {
	return s == SessionPlanStatusCompleted || s == SessionPlanStatusSkipped
}

// Phase is a training phase within the 4-week periodization model.
type Phase string

const (
	PhaseAccumulation Phase = "ACCUMULATION"
	PhaseOverload     Phase = "OVERLOAD"
	PhasePeak         Phase = "PEAK"
	PhaseDeload       Phase = "DELOAD"
)

// Valid reports whether p is a known Phase.
func (p Phase) Valid() bool {
	switch p {
	case PhaseAccumulation, PhaseOverload, PhasePeak, PhaseDeload:
		return true
	default:
		return false
	}
}

// MaxSessionsPerWeek is the hard cap defined by BR-AC-01.
const MaxSessionsPerWeek = 6

// WeeksPerRoadmap is the fixed 4-week program length.
const WeeksPerRoadmap = 4

// DaysPerWeek is the fixed 7-day allocation per WeekPlan.
const DaysPerWeek = 7
