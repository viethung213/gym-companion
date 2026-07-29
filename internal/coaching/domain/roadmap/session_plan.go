package roadmap

import (
	"fmt"
	"strings"
	"time"
)

// PrescribedExercise is a single exercise line item within a WorkoutPrescription.
type PrescribedExercise struct {
	ExerciseID      string  `json:"exercise_id"`
	ExerciseName    string  `json:"exercise_name"`
	TargetSets      int32   `json:"target_sets"`
	TargetReps      int32   `json:"target_reps"`
	TargetWeight    float32 `json:"target_weight"`
	DurationSeconds int32   `json:"duration_seconds"`
	Notes           string  `json:"notes"`
	RestSetSec      int32   `json:"rest_set_sec"`
	RestExerciseSec int32   `json:"rest_exercise_sec"`
	TargetRPE       float32 `json:"target_rpe"`
}

// WorkoutPrescription is the value object describing the workout script.
type WorkoutPrescription struct {
	WarmUps       []PrescribedExercise `json:"warm_ups"`
	MainExercises []PrescribedExercise `json:"main_exercises"`
	CoolDowns     []PrescribedExercise `json:"cool_downs"`
}

// SessionPlanInfo is the value-object snapshot of a SessionPlan entity.
type SessionPlanInfo struct {
	SessionPlanID       string
	DayPlanID           string
	WeekPlanID          string
	RoadmapID           string
	UserID              string
	ScheduledDate       time.Time
	SlotTime            string
	Status              SessionPlanStatus
	TargetMuscleGroups  []string
	Prescription        WorkoutPrescription
	Reasoning           string
	GeneratedAt         time.Time
	CompletedAt         *time.Time
	SessionSCR          *float32
	SessionDeltaRPE     *float32
}

// SessionPlan is an entity nested under DayPlan.
type SessionPlan struct {
	info SessionPlanInfo
}

// NewSessionPlan constructs a SessionPlan in PENDING status.
//nolint:gocritic // hugeParam: SessionPlanInfo snapshot passed by value
func NewSessionPlan(info SessionPlanInfo, now time.Time) (*SessionPlan, error) {
	info = normalizeSessionInfo(info)
	info.Status = SessionPlanStatusPending
	info.GeneratedAt = now
	info.CompletedAt = nil
	info.SessionSCR = nil
	info.SessionDeltaRPE = nil

	sp := &SessionPlan{info: info}
	if err := sp.validate(); err != nil {
		return nil, err
	}
	return sp, nil
}

// RehydrateSessionPlan loads a SessionPlan from persistence without applying
// lifecycle defaults.
//nolint:gocritic // hugeParam: SessionPlanInfo snapshot passed by value
func RehydrateSessionPlan(info SessionPlanInfo) (*SessionPlan, error) {
	info = normalizeSessionInfo(info)
	if !info.Status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, info.Status)
	}

	sp := &SessionPlan{info: info}
	if err := sp.validate(); err != nil {
		return nil, err
	}
	return sp, nil
}

// Info returns a deep copy of the session plan snapshot.
func (s *SessionPlan) Info() SessionPlanInfo {
	info := s.info
	info.TargetMuscleGroups = copyStringSlice(s.info.TargetMuscleGroups)
	info.Prescription = copyPrescription(s.info.Prescription)
	if s.info.CompletedAt != nil {
		t := *s.info.CompletedAt
		info.CompletedAt = &t
	}
	if s.info.SessionSCR != nil {
		v := *s.info.SessionSCR
		info.SessionSCR = &v
	}
	if s.info.SessionDeltaRPE != nil {
		v := *s.info.SessionDeltaRPE
		info.SessionDeltaRPE = &v
	}
	return info
}

// ID returns the session plan identifier.
func (s *SessionPlan) ID() string { return s.info.SessionPlanID }

// Status returns the current lifecycle status.
func (s *SessionPlan) Status() SessionPlanStatus { return s.info.Status }

// ScheduledDate returns the calendar date the session is planned for.
func (s *SessionPlan) ScheduledDate() time.Time { return s.info.ScheduledDate }

// MarkCompleted transitions PENDING → COMPLETED and records execution metrics.
// scr is Session Completion Rate (0-100); deltaRPE is scalar per D7.
// Idempotent: calling on an already-COMPLETED session is a no-op.
func (s *SessionPlan) MarkCompleted(scr, deltaRPE float32, now time.Time) error {
	if s.info.Status == SessionPlanStatusCompleted {
		return nil
	}
	if s.info.Status != SessionPlanStatusPending {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, s.info.Status, SessionPlanStatusCompleted)
	}
	s.info.Status = SessionPlanStatusCompleted
	completedAt := now
	s.info.CompletedAt = &completedAt
	scrCopy := scr
	deltaCopy := deltaRPE
	s.info.SessionSCR = &scrCopy
	s.info.SessionDeltaRPE = &deltaCopy
	return nil
}

// MarkSkipped transitions PENDING → SKIPPED. Used both for automatic skipping
// when scheduled_date passes and for WorkoutSessionAborted events.
// Idempotent: calling on an already-SKIPPED session is a no-op.
func (s *SessionPlan) MarkSkipped() error {
	if s.info.Status == SessionPlanStatusSkipped {
		return nil
	}
	if s.info.Status != SessionPlanStatusPending {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, s.info.Status, SessionPlanStatusSkipped)
	}
	s.info.Status = SessionPlanStatusSkipped
	return nil
}

// RewritePrescription replaces the prescription content in-place while the
// session is still PENDING (D3: Regenerate touches only PENDING sessions).
func (s *SessionPlan) RewritePrescription(p WorkoutPrescription, muscleGroups []string, reasoning string, now time.Time) error {
	if s.info.Status != SessionPlanStatusPending {
		return fmt.Errorf("%w: cannot rewrite session in status %s", ErrSessionAlreadyFinal, s.info.Status)
	}
	s.info.Prescription = copyPrescription(p)
	s.info.TargetMuscleGroups = copyStringSlice(muscleGroups)
	s.info.Reasoning = strings.TrimSpace(reasoning)
	s.info.GeneratedAt = now
	return nil
}

func (s *SessionPlan) validate() error {
	i := s.info
	if i.SessionPlanID == "" {
		return fmt.Errorf("%w: session_plan_id is required", ErrInvalidRoadmap)
	}
	if i.DayPlanID == "" {
		return fmt.Errorf("%w: day_plan_id is required", ErrInvalidRoadmap)
	}
	if i.UserID == "" {
		return fmt.Errorf("%w: user_id is required", ErrInvalidRoadmap)
	}
	if i.ScheduledDate.IsZero() {
		return fmt.Errorf("%w: scheduled_date is required", ErrInvalidRoadmap)
	}
	if !i.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, i.Status)
	}
	return nil
}

//nolint:gocritic // hugeParam: SessionPlanInfo snapshot passed by value
func normalizeSessionInfo(info SessionPlanInfo) SessionPlanInfo {
	info.SessionPlanID = strings.TrimSpace(info.SessionPlanID)
	info.DayPlanID = strings.TrimSpace(info.DayPlanID)
	info.WeekPlanID = strings.TrimSpace(info.WeekPlanID)
	info.RoadmapID = strings.TrimSpace(info.RoadmapID)
	info.UserID = strings.TrimSpace(info.UserID)
	info.SlotTime = strings.TrimSpace(info.SlotTime)
	info.Reasoning = strings.TrimSpace(info.Reasoning)
	info.TargetMuscleGroups = copyStringSlice(info.TargetMuscleGroups)
	info.Prescription = copyPrescription(info.Prescription)
	return info
}

func copyPrescription(p WorkoutPrescription) WorkoutPrescription {
	return WorkoutPrescription{
		WarmUps:       copyExercises(p.WarmUps),
		MainExercises: copyExercises(p.MainExercises),
		CoolDowns:     copyExercises(p.CoolDowns),
	}
}

func copyExercises(in []PrescribedExercise) []PrescribedExercise {
	if in == nil {
		return nil
	}
	out := make([]PrescribedExercise, len(in))
	copy(out, in)
	return out
}

func copyStringSlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
