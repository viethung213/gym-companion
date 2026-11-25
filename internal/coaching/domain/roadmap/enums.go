// Package roadmap contains the Roadmap aggregate and its child entities
// (WeekPlan, DayPlan, SessionPlan) with 4-tier DDD structure.
package roadmap

// Status is the lifecycle status of a Roadmap aggregate.
type Status string

const (
	// StatusPlanned is the next 4-week block, drafted ahead and waiting for the
	// current roadmap to end.
	StatusPlanned   Status = "PLANNED"
	StatusActive    Status = "ACTIVE"
	StatusCompleted Status = "COMPLETED"
)

// Valid reports whether s is a known Status.
func (s Status) Valid() bool {
	switch s {
	case StatusPlanned, StatusActive, StatusCompleted:
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
	SessionPlanStatusAborted   SessionPlanStatus = "ABORTED"
)

// Valid reports whether s is a known SessionPlanStatus.
func (s SessionPlanStatus) Valid() bool {
	switch s {
	case SessionPlanStatusPending, SessionPlanStatusCompleted, SessionPlanStatusSkipped, SessionPlanStatusAborted:
		return true
	default:
		return false
	}
}

// IsFinal reports whether the session has reached a terminal state.
func (s SessionPlanStatus) IsFinal() bool {
	return s == SessionPlanStatusCompleted || s == SessionPlanStatusSkipped || s == SessionPlanStatusAborted
}

// SessionPlanSource defines the origin of a SessionPlan.
type SessionPlanSource string

const (
	SessionPlanSourceScheduled SessionPlanSource = "COACH_SCHEDULED"
	SessionPlanSourceAdHoc     SessionPlanSource = "USER_ADHOC"
	SessionPlanSourceSoloMatch SessionPlanSource = "SOLO_MATCH"
)

// Valid reports whether s is a known SessionPlanSource.
func (s SessionPlanSource) Valid() bool {
	switch s {
	case SessionPlanSourceScheduled, SessionPlanSourceAdHoc, SessionPlanSourceSoloMatch:
		return true
	default:
		return false
	}
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

// MaxSessionsPerWeek is the hard cap on TRAINING sessions defined by BR-AC-01.
// Recovery sessions do not count toward it.
const MaxSessionsPerWeek = 6

// MaxSlotsPerWeek caps all sessions of a week, training and recovery: the
// athlete may select at most this many weekly slots.
const MaxSlotsPerWeek = 10

// RecoveryMaxRPE is the intensity ceiling of a recovery session.
const RecoveryMaxRPE = 4

// SessionKind separates training sessions from active-recovery ones.
type SessionKind string

const (
	SessionKindTraining SessionKind = "TRAINING"
	// SessionKindRecovery is mobility, stretching or light cardio at RPE <= RecoveryMaxRPE.
	SessionKindRecovery SessionKind = "RECOVERY"
)

// Valid reports whether k is a known SessionKind.
func (k SessionKind) Valid() bool { return k == SessionKindTraining || k == SessionKindRecovery }

// WeeksPerRoadmap is the fixed 4-week program length.
const WeeksPerRoadmap = 4

// DaysPerWeek is the fixed 7-day allocation per WeekPlan.
const DaysPerWeek = 7
