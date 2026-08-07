package adk

import "time"

// InjuryStatus represents the severity and muscle group of a user's injury.
type InjuryStatus struct {
	MuscleGroup string `json:"muscle_group"`
	Severity    string `json:"severity"` // "MILD" | "MODERATE" | "SEVERE"
}

// WorkoutSlot defines available time windows for training.
type WorkoutSlot struct {
	DayOfWeek int    `json:"day_of_week"` // 0=Monday ... 6=Sunday
	StartTime string `json:"start_time"`  // "HH:MM"
	EndTime   string `json:"end_time"`    // "HH:MM"
}

// WorkoutSession represents historical session execution metrics.
type WorkoutSession struct {
	SessionID    string    `json:"session_id"`
	CompletedAt  time.Time `json:"completed_at"`
	MuscleGroups []string  `json:"muscle_groups"`
	AverageRPE   float64   `json:"average_rpe"`
	TotalSets    int       `json:"total_sets"`
	Aborted      bool      `json:"aborted"`
}

// UserProfile aggregates user metrics, preferences, equipment and active injuries.
type UserProfile struct {
	UserID                string         `json:"user_id"`
	WeightKg              float64        `json:"weight_kg"`
	PrimaryGoal           string         `json:"primary_goal"`
	AvailableEquipment    []string       `json:"available_equipment"`
	PreferredMuscleGroups []string       `json:"preferred_muscle_groups"`
	AvailableSlots        []WorkoutSlot  `json:"available_slots"`
	ActiveInjuries        []InjuryStatus `json:"active_injuries"`
}

// RoadmapSnapshot represents the active roadmap snapshot.
type RoadmapSnapshot struct {
	RoadmapID         string   `json:"roadmap_id"`
	CurrentWeek       int      `json:"current_week"`
	Phase             string   `json:"phase"` // "ACCUMULATION" | "OVERLOAD" | "PEAK" | "DELOAD"
	PendingSessionIDs []string `json:"pending_session_ids"`
}

// CoachInput represents the full typed input context sent to the agent.
type CoachInput struct {
	Flow           string           `json:"flow"`
	CurrentTime    string           `json:"current_time"` // ISO-8601 string
	Profile        UserProfile      `json:"profile"`
	RecentSessions []WorkoutSession `json:"recent_sessions"`
	CurrentRoadmap *RoadmapSnapshot `json:"current_roadmap,omitempty"`

	// 1-based; >1 means the previous plan was rejected. omitempty keeps a first
	// attempt's payload identical to one with no retry support.
	AttemptNumber int `json:"attempt_number,omitempty"`

	// PriorAttemptIssues are deterministic defects from domain validation.
	PriorAttemptIssues []string `json:"prior_attempt_issues,omitempty"`

	// PreviousPlan is the output being revised. Without it "keep what was not
	// flagged" is unactionable: a node-wrapped agent gets no conversation
	// history, so the model cannot see its own previous answer.
	PreviousPlan *GeneratedPlan `json:"previous_plan,omitempty"`

	// ReviewFeedback is the reviewer's verdict on PreviousPlan. Flow and
	// Profile already carry the original task and user context, so neither is
	// repeated here.
	ReviewFeedback *PlanReview `json:"review_feedback,omitempty"`

	SessionsToRevise []SessionToRevise `json:"sessions_to_revise,omitempty"`
	AdaptationReason string            `json:"adaptation_reason,omitempty"`
}

// SessionToRevise carries no id; results are matched back by position.
type SessionToRevise struct {
	ScheduledDate       string              `json:"scheduled_date"` // YYYY-MM-DD
	TargetMuscleGroups  []string            `json:"target_muscle_groups"`
	CurrentPrescription WorkoutPrescription `json:"current_prescription"`
	CurrentReasoning    string              `json:"current_reasoning,omitempty"`
}

// PrescribedExercise instructs one exercise. No exercise_name: a hallucinated
// name would persist beside the ID with nothing to reconcile it against.
type PrescribedExercise struct {
	ExerciseID     string  `json:"exercise_id"`
	TargetSets     int     `json:"target_sets"`
	TargetReps     int     `json:"target_reps"`
	TargetWeightKg float64 `json:"target_weight_kg"`
	TargetRPE      float64 `json:"target_rpe"`
	RestSetSec     int     `json:"rest_set_sec"`
}

// WorkoutPrescription represents the structure of one session prescription.
type WorkoutPrescription struct {
	WarmUps       []PrescribedExercise `json:"warm_ups"`
	MainExercises []PrescribedExercise `json:"main_exercises"`
	CoolDowns     []PrescribedExercise `json:"cool_downs"`
}

// SessionPlan is one scheduled session. No session_plan_id: the backend mints
// identifiers, so the model cannot collide with or point at another user's row.
type SessionPlan struct {
	ScheduledDate      string              `json:"scheduled_date"` // YYYY-MM-DD
	TargetMuscleGroups []string            `json:"target_muscle_groups"`
	Prescription       WorkoutPrescription `json:"prescription"`
	Reasoning          string              `json:"reasoning"`
}

// WeekPlan defines a single week inside the 4-week block.
type WeekPlan struct {
	WeekNumber   int           `json:"week_number"`
	Phase        string        `json:"phase"` // "ACCUMULATION" | "OVERLOAD" | "PEAK" | "DELOAD"
	TargetRPEMin float64       `json:"target_rpe_min"`
	TargetRPEMax float64       `json:"target_rpe_max"`
	Sessions     []SessionPlan `json:"sessions"`
}

// GeneratedPlan is the generated roadmap. No user_id: an LLM-controlled one
// reaching a domain aggregate would be a cross-tenant write primitive.
type GeneratedPlan struct {
	Weeks []WeekPlan `json:"weeks"`
}

// PlanResult is a catalog-validated plan plus resolved names and a loss marker.
type PlanResult struct {
	Plan     *GeneratedPlan    `json:"plan"`
	Names    map[string]string `json:"-"` // exercise_id -> catalog display name
	Degraded bool              `json:"degraded"`
	Issues   []string          `json:"issues,omitempty"`

	// Review is the last verdict the plan received, nil when the flow ships
	// unreviewed. A shipped plan may carry a failing verdict: the loop ran out
	// of rounds and the guardrail, not the reviewer, is the hard gate.
	Review *PlanReview `json:"review,omitempty"`
}

// EvaluationResult represents the output of quality evaluation step.
type EvaluationResult struct {
	IsValid  bool              `json:"is_valid"`
	Issues   []string          `json:"issues"`
	Plan     GeneratedPlan     `json:"plan"`
	Names    map[string]string `json:"-"` // exercise_id -> catalog display name
	Degraded bool              `json:"degraded"`
	Review   *PlanReview       `json:"review,omitempty"`
}

// ValidationReport is what domain validation concluded about a candidate plan.
// The reviewer reads it so it does not re-litigate catalog membership or the
// weekly caps, which are already settled deterministically.
type ValidationReport struct {
	Passed   bool     `json:"passed"`
	Degraded bool     `json:"degraded"` // salvage mode dropped something
	Issues   []string `json:"issues,omitempty"`
}

// ReviewRequest is the reviewer's complete input. All four parts are named
// explicitly so the prompt can refer to them and the model cannot confuse the
// plan it is judging with the context it is judging against.
type ReviewRequest struct {
	OriginalTask     string           `json:"original_task"`
	UserContext      UserProfile      `json:"user_context"`
	RecentSessions   []WorkoutSession `json:"recent_sessions,omitempty"`
	GeneratorOutput  *GeneratedPlan   `json:"generator_output"`
	ValidationResult ValidationReport `json:"validation_result"`
	ReviewRound      int              `json:"review_round"`
}

// ReviewNote is one actionable defect. Area locates it, Fix says what to do;
// a note without Fix sends the generator back with nothing to act on.
type ReviewNote struct {
	Area   string `json:"area"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// PlanReview is the reviewer's verdict. It deliberately carries no plan field:
// the reviewer may judge, score and advise, but never rewrite. A corrected
// plan can only come from the generator, which is the only path that then goes
// back through domain validation.
type PlanReview struct {
	Approved   bool         `json:"approved"`
	Score      int          `json:"score"`      // 0..100
	Confidence float64      `json:"confidence"` // 0..1
	Feedback   []ReviewNote `json:"feedback,omitempty"`
}
