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
	SessionPlanID      string
	DayPlanID          string
	WeekPlanID         string
	RoadmapID          string
	UserID             string
	ScheduledDate            time.Time
	SlotTime                 string
	EstimatedDurationMinutes int32
	Status                   SessionPlanStatus
	Source             SessionPlanSource
	TargetMuscleGroups []string
	Prescription       WorkoutPrescription
	Reasoning          string
	GeneratedAt        time.Time
	CompletedAt        *time.Time
	SessionSCR         *float32
	SessionDeltaRPE    *float32
}

// SessionPlan is an entity nested under DayPlan.
type SessionPlan struct {
	info SessionPlanInfo
}

// NewSessionPlan constructs a SessionPlan in PENDING status.
func NewSessionPlan(info *SessionPlanInfo, now time.Time) (*SessionPlan, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil SessionPlanInfo", ErrInvalidRoadmap)
	}

	normInfo := normalizeSessionInfo(info)

	normInfo.Status = SessionPlanStatusPending

	if normInfo.Source == "" {
		normInfo.Source = SessionPlanSourceScheduled
	}

	normInfo.GeneratedAt = now

	normInfo.CompletedAt = nil

	normInfo.SessionSCR = nil

	normInfo.SessionDeltaRPE = nil

	sp := &SessionPlan{info: *normInfo}

	if err := sp.validate(); err != nil {
		return nil, err
	}

	return sp, nil
}

// RehydrateSessionPlan loads a SessionPlan from persistence without applying
// lifecycle defaults.
func RehydrateSessionPlan(info *SessionPlanInfo) (*SessionPlan, error) {
	if info == nil {
		return nil, fmt.Errorf("%w: nil SessionPlanInfo", ErrInvalidRoadmap)
	}

	normInfo := normalizeSessionInfo(info)

	if !normInfo.Status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, normInfo.Status)
	}

	sp := &SessionPlan{info: *normInfo}

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

// MarkSkipped transitions PENDING → SKIPPED when scheduled_date passes
// without a session being started.
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

// MarkAborted transitions PENDING → ABORTED for mid-workout cancellations
// or 4-hour inactivity timeouts (WorkoutSessionAborted event).
// Idempotent: calling on an already-ABORTED session is a no-op.
func (s *SessionPlan) MarkAborted() error {
	if s.info.Status == SessionPlanStatusAborted {
		return nil
	}

	if s.info.Status != SessionPlanStatusPending {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, s.info.Status, SessionPlanStatusAborted)
	}

	s.info.Status = SessionPlanStatusAborted

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

	if i.Source != "" && !i.Source.Valid() {
		return fmt.Errorf("%w: invalid source %s", ErrInvalidRoadmap, i.Source)
	}

	return nil
}

func normalizeSessionInfo(info *SessionPlanInfo) *SessionPlanInfo {
	if info == nil {
		return &SessionPlanInfo{}
	}

	cp := *info

	cp.SessionPlanID = strings.TrimSpace(cp.SessionPlanID)

	cp.DayPlanID = strings.TrimSpace(cp.DayPlanID)

	cp.WeekPlanID = strings.TrimSpace(cp.WeekPlanID)

	cp.RoadmapID = strings.TrimSpace(cp.RoadmapID)

	cp.UserID = strings.TrimSpace(cp.UserID)

	cp.SlotTime = strings.TrimSpace(cp.SlotTime)

	cp.Reasoning = strings.TrimSpace(cp.Reasoning)

	cp.TargetMuscleGroups = copyStringSlice(cp.TargetMuscleGroups)

	cp.Prescription = copyPrescription(cp.Prescription)

	return &cp
}

func copyPrescription(p WorkoutPrescription) WorkoutPrescription {
	return WorkoutPrescription{
		WarmUps: copyExercises(p.WarmUps),

		MainExercises: copyExercises(p.MainExercises),

		CoolDowns: copyExercises(p.CoolDowns),
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
