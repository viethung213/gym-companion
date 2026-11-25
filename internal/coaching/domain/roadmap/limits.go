package roadmap

import "fmt"

// Hard limits on a prescription line (BR-AC-13). They bound what any session
// may ask of an athlete, whoever wrote it, so a model slip such as 30 sets or
// 400 kg cannot reach the athlete. Tighter, athlete-specific limits (injury
// RPE caps, at most 30% above a personal record) apply on top.
const (
	MaxTargetSets      = 10
	MaxTargetReps      = 50
	MaxDurationSeconds = 30 * 60
	MaxRestSetSec      = 10 * 60
	MaxTargetRPE       = 10
	MaxTargetWeightKg  = 300

	MinSessionMinutes = 10
	MaxSessionMinutes = 180
)

// UnrecordedLoadFactor caps the load of an exercise the athlete has no
// personal record for, as a multiple of bodyweight. Without history the load is
// a guess, and the guess must not be one that hurts a beginner; once a record
// exists the 30% ceiling above it (BR-AC-02) applies instead.
const UnrecordedLoadFactor = 1.5

// LimitIssue describes the first hard limit the exercise breaks, or "" when it
// respects them all. These are safety limits: a value that is negative or asks
// too much. A value left out (0 sets, no RPE) is incomplete rather than unsafe;
// the generation validator requires those for new plans, and a line already
// stored with one must not block every later rewrite of the roadmap.
func (e *PrescribedExercise) LimitIssue() string {
	switch {
	case e.TargetSets < 0 || e.TargetSets > MaxTargetSets:
		return fmt.Sprintf("target_sets %d outside 0..%d", e.TargetSets, MaxTargetSets)
	case e.TargetReps < 0 || e.TargetReps > MaxTargetReps:
		return fmt.Sprintf("target_reps %d outside 0..%d", e.TargetReps, MaxTargetReps)
	case e.DurationSeconds < 0 || e.DurationSeconds > MaxDurationSeconds:
		return fmt.Sprintf("duration_seconds %d outside 0..%d", e.DurationSeconds, MaxDurationSeconds)
	case e.RestSetSec < 0 || e.RestSetSec > MaxRestSetSec:
		return fmt.Sprintf("rest_set_sec %d outside 0..%d", e.RestSetSec, MaxRestSetSec)
	case e.TargetWeight < 0 || e.TargetWeight > MaxTargetWeightKg:
		return fmt.Sprintf("target_weight %.1f kg outside 0..%d", e.TargetWeight, MaxTargetWeightKg)
	case e.TargetRPE < 0 || e.TargetRPE > MaxTargetRPE:
		return fmt.Sprintf("target_rpe %.1f outside 0..%d", e.TargetRPE, MaxTargetRPE)
	default:
		return ""
	}
}

// SessionDurationIssue describes why a session length is out of bounds, or "".
// Zero means "not stated" (a user-built ad-hoc session) and passes; the AI
// output validator separately requires at least MinSessionMinutes.
func SessionDurationIssue(minutes int32) string {
	if minutes < 0 || minutes > MaxSessionMinutes {
		return fmt.Sprintf("estimated_duration_minutes %d outside 0..%d", minutes, MaxSessionMinutes)
	}
	return ""
}
