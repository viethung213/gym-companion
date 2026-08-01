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
	AttemptNumber      int      `json:"attempt_number,omitempty"`
	PriorAttemptIssues []string `json:"prior_attempt_issues,omitempty"`

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
}

// EvaluationResult represents the output of quality evaluation step.
type EvaluationResult struct {
	IsValid  bool              `json:"is_valid"`
	Issues   []string          `json:"issues"`
	Plan     GeneratedPlan     `json:"plan"`
	Names    map[string]string `json:"-"` // exercise_id -> catalog display name
	Degraded bool              `json:"degraded"`
}
