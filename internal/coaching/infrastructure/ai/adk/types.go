package adk

import (
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// InjuryStatus is one active injury, or an ad-hoc avoid, as taxonomy codes.
type InjuryStatus struct {
	Region   string `json:"region,omitempty"` // JointRegion code
	Muscle   string `json:"muscle,omitempty"` // MuscleGroup code
	Severity string `json:"severity"`         // "MILD" | "MODERATE" | "SEVERE"
}

// WorkoutSlot defines available time windows for training.
type WorkoutSlot struct {
	DayOfWeek int    `json:"day_of_week"` // time.Weekday: 0=Sunday ... 6=Saturday
	StartTime string `json:"start_time"`  // "HH:MM"
	EndTime   string `json:"end_time"`    // "HH:MM"
}

// WeekSlotPlan lists the selected slots still open in one week of the initial
// roadmap and how many sessions that week must hold.
type WeekSlotPlan struct {
	WeekNumber       int         `json:"week_number"`
	ExpectedSessions int         `json:"expected_sessions"`
	TrainingSessions int         `json:"training_sessions"`
	Slots            []DatedSlot `json:"slots"`
}

// DatedSlot is one occurrence of a selected slot on a calendar date.
type DatedSlot struct {
	ScheduledDate string `json:"scheduled_date"` // YYYY-MM-DD
	SlotTime      string `json:"slot_time"`      // as in preferred_workout_times
	SessionKind   string `json:"session_kind"`   // "TRAINING" | "RECOVERY", assigned by the backend
	// Phase is the roadmap phase of the week the slot falls in; rewrites only.
	Phase string `json:"-"`
}

// UserProfile aggregates user metrics, preferences, equipment and active injuries.
type UserProfile struct {
	UserID                string              `json:"user_id"`
	WeightKg              float64             `json:"weight_kg"`
	ExperienceLevel       string              `json:"experience_level,omitempty"`
	PrimaryGoal           string              `json:"primary_goal"`
	AvailableEquipment    []string            `json:"available_equipment"`     // Equipment codes
	PreferredMuscleGroups []string            `json:"preferred_muscle_groups"` // MuscleGroup codes
	PreferredWorkoutTimes map[string][]string `json:"preferred_workout_times"`
	AvailableSlots        []WorkoutSlot       `json:"available_slots"`
	ActiveInjuries        []InjuryStatus      `json:"active_injuries"`
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
	Flow        string `json:"flow"`
	CurrentTime string `json:"current_time"` // RFC 3339, in the athlete's zone
	// Timezone is the athlete's IANA zone: scheduled_date and slot_time are wall clock in it.
	Timezone       string              `json:"timezone,omitempty"`
	Profile        UserProfile         `json:"profile"`
	RecentSessions []string            `json:"recent_sessions"`
	MusclePRs      map[string][]string `json:"muscle_prs,omitempty"`
	CurrentRoadmap *RoadmapSnapshot    `json:"current_roadmap,omitempty"`
	// AvailableExercises groups compact 6-element main exercise tuples by
	// primary MuscleGroup code:
	// [exercise_id, name, equipment, level, movement_type, measurement_type].
	AvailableExercises map[string][][]string `json:"available_exercises,omitempty"`
	// RecoveryExercises groups STRETCHING exercises, used for both warm-ups and
	// cool-downs, by primary MuscleGroup code (+ "General").
	RecoveryExercises map[string][][]string `json:"recovery_exercises,omitempty"`
	// InjuryLimits maps exercise_id to the injury limit it must respect
	// (RPE cap and reason) for exercises kept despite an injury.
	InjuryLimits map[string]string `json:"injury_limits,omitempty"`

	// WeekSlots is set for INITIATE_4_WEEK only: every week's open slots and
	// the session count it must hold.
	WeekSlots []WeekSlotPlan `json:"week_slots,omitempty"`

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
	// Rescheduled marks sessions_to_revise as new sessions on changed weekly slots.
	Rescheduled      bool            `json:"rescheduled,omitempty"`
	AdaptationReason string          `json:"adaptation_reason,omitempty"`
	AdHocHint        *AdHocHintInput `json:"adhoc_hint,omitempty"`
}

// AdHocHintInput defines custom user preferences for single ad-hoc session recommendations.
type AdHocHintInput struct {
	FreeText           string   `json:"free_text,omitempty"`
	MuscleGroups       []string `json:"muscle_groups,omitempty"`
	AvailableEquipment []string `json:"available_equipment,omitempty"`
	DurationMinutes    int      `json:"duration_minutes,omitempty"`
	IntensityHint      string   `json:"intensity_hint,omitempty"`
}

// SessionToRevise carries no id; results are matched back by position.
type SessionToRevise struct {
	ScheduledDate string `json:"scheduled_date"` // YYYY-MM-DD
	SlotTime      string `json:"slot_time"`
	SessionKind   string `json:"session_kind"`
	// Phase is the phase of the roadmap week the session falls in: its TRAINING
	// main work must sit in that phase's RPE window.
	Phase                    string              `json:"phase,omitempty"`
	EstimatedDurationMinutes int32               `json:"estimated_duration_minutes"`
	TargetMuscleGroups       []string            `json:"target_muscle_groups"`
	CurrentPrescription      WorkoutPrescription `json:"current_prescription"`
	CurrentReasoning         string              `json:"current_reasoning,omitempty"`
}

// PrescribedExercise instructs one exercise. No exercise_name: a hallucinated
// name would persist beside the ID with nothing to reconcile it against.
type PrescribedExercise struct {
	ExerciseID      string  `json:"exercise_id"`
	TargetSets      int     `json:"target_sets"`
	TargetReps      int     `json:"target_reps,omitempty"`
	DurationSeconds int     `json:"duration_seconds,omitempty"`
	TargetWeightKg  float64 `json:"target_weight_kg"`
	TargetRPE       float64 `json:"target_rpe"`
	RestSetSec      int     `json:"rest_set_sec"`
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
	ScheduledDate            string              `json:"scheduled_date"`         // YYYY-MM-DD
	SlotTime                 string              `json:"slot_time"`              // "HH:MM" or "06:00-07:30"
	SessionKind              string              `json:"session_kind,omitempty"` // pinned by the backend from the slot
	EstimatedDurationMinutes int32               `json:"estimated_duration_minutes"`
	TargetMuscleGroups       []string            `json:"target_muscle_groups"`
	Prescription             WorkoutPrescription `json:"prescription"`
	Reasoning                string              `json:"reasoning,omitempty"`

	// phase is pinned by the backend before validation (the week's phase, or
	// the revised session's); empty for flows without periodization (ad-hoc).
	// Unexported, so it never reaches or comes from the model.
	phase roadmap.Phase
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
	Cautions map[string]string `json:"-"` // exercise_id -> injury caution kept in reasoning
	// Primaries maps each validated exercise_id to its primary MuscleGroup code.
	Primaries map[string]string `json:"-"`
	Degraded  bool              `json:"degraded"`
	Issues    []string          `json:"issues,omitempty"`
	// Timezone is the zone the plan was scheduled in (CoachInput.Timezone).
	Timezone string `json:"-"`
	// WeekStart is week 1's first calendar day as the slot schedule counted it
	// (the plan's creation day in Timezone); zero when no schedule applied.
	WeekStart time.Time `json:"-"`

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
	RecentSessions   []string         `json:"recent_sessions,omitempty"`
	GeneratorOutput  *GeneratedPlan   `json:"generator_output"`
	ValidationResult ValidationReport `json:"validation_result"`
	ReviewRound      int              `json:"review_round"`

	// PreviousFeedback is what the last round asked the generator to change.
	// Without it a reviewer judges each plan as if no round had happened, so a
	// fix the generator quietly skipped reads as a clean plan and gets approved.
	PreviousFeedback []ReviewNote `json:"previous_feedback,omitempty"`
}

// ReviewNote is one actionable defect. Area locates it, Fix says what to do;
// a note without Fix sends the generator back with nothing to act on.
type ReviewNote struct {
	Area   string `json:"area"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// FeedbackOutcome reports what became of one item the previous round asked for.
// Evidence is required either way: "applied" without a location is a claim, not
// a check.
type FeedbackOutcome struct {
	Area     string `json:"area"`
	Applied  bool   `json:"applied"`
	Evidence string `json:"evidence"`
}

// PlanReview is the reviewer's verdict. It deliberately carries no plan field:
// the reviewer may judge, score and advise, but never rewrite. A corrected
// plan can only come from the generator, which is the only path that then goes
// back through domain validation.
type PlanReview struct {
	Approved   bool    `json:"approved"`
	Score      int     `json:"score"`      // 0..100
	Confidence float64 `json:"confidence"` // 0..1

	// Feedback is strictly must-fix. Anything the reviewer merely likes or
	// wonders about belongs in Notes: mixing the two turns Fix into praise
	// ("keep doing this"), which passes validation while telling the generator
	// nothing to do.
	Feedback []ReviewNote `json:"feedback,omitempty"`
	Notes    []string     `json:"notes,omitempty"`

	// PreviousFeedback answers, item by item, whether the last round's fixes
	// actually landed.
	PreviousFeedback []FeedbackOutcome `json:"previous_feedback,omitempty"`
}

// unappliedAreas lists the previous round's items the reviewer reports as still
// not done.
func (v *PlanReview) unappliedAreas() []string {
	if v == nil {
		return nil
	}

	var out []string
	for _, o := range v.PreviousFeedback {
		if !o.Applied {
			out = append(out, o.Area)
		}
	}
	return out
}
