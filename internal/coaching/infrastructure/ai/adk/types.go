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
}

// PrescribedExercise is a specific instruction for an exercise.
type PrescribedExercise struct {
	ExerciseID     string  `json:"exercise_id"`
	ExerciseName   string  `json:"exercise_name"`
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

// SessionPlan represents a single scheduled training session plan.
type SessionPlan struct {
	SessionPlanID      string              `json:"session_plan_id"`
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

// GeneratedPlan contains the complete generated roadmap.
type GeneratedPlan struct {
	UserID string     `json:"user_id"`
	Weeks  []WeekPlan `json:"weeks"`
}

// EvaluationResult represents the output of quality evaluation step.
type EvaluationResult struct {
	IsValid bool          `json:"is_valid"`
	Issues  []string      `json:"issues"`
	Plan    GeneratedPlan `json:"plan"`
}
