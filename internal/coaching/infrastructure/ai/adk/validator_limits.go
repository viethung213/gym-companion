package adk

import (
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// defaultMainRPE fills a main exercise the model left without intensity, in a
// flow with no phase window to take it from (ad-hoc).
const defaultMainRPE = 7.0

// clampLine checks one prescription line against the numeric limits the
// guardrail enforces later, so the model hears about a breach while it can
// still fix it instead of the whole plan failing at the hard gate. It returns
// one reason per breach and clamps ex into every limit, which is what salvage
// ships when the model runs out of attempts.
//
// measure is the catalog's measurement type ("REPS" | "DURATION"); injuryCap
// is the RPE cap an injury puts on this exercise, 0 when none.
func (v *planValidator) clampLine(ex *PrescribedExercise, main bool, measure string, sp *SessionPlan, injuryCap float64) []string {
	reasons := clampMeasure(ex, main, measure)
	clampInt := func(field string, val *int, lo, hi int) {
		if *val < lo || *val > hi {
			reasons = append(reasons, fmt.Sprintf("%s %d must be %d to %d", field, *val, lo, hi))
			*val = min(max(*val, lo), hi)
		}
	}
	clampInt("target_sets", &ex.TargetSets, 1, roadmap.MaxTargetSets)
	clampInt("target_reps", &ex.TargetReps, 0, roadmap.MaxTargetReps)
	clampInt("duration_seconds", &ex.DurationSeconds, 0, roadmap.MaxDurationSeconds)
	clampInt("rest_set_sec", &ex.RestSetSec, 0, roadmap.MaxRestSetSec)

	reasons = append(reasons, clampRPE(ex, main, sp, injuryCap)...)

	if reason := v.clampLoad(ex); reason != "" {
		reasons = append(reasons, reason)
	}
	return reasons
}

// clampRPE keeps target_rpe on the 0-10 scale, requires it on main work, and
// holds a TRAINING session's main work to its phase window (BR-AC-10), lowered
// by any injury cap. Recovery sessions are already capped by capRecoveryRPE.
func clampRPE(ex *PrescribedExercise, main bool, sp *SessionPlan, injuryCap float64) []string {
	var reasons []string
	if ex.TargetRPE < 0 || ex.TargetRPE > roadmap.MaxTargetRPE {
		reasons = append(reasons, fmt.Sprintf("target_rpe %.1f must be 0 to %d", ex.TargetRPE, roadmap.MaxTargetRPE))
		ex.TargetRPE = min(max(ex.TargetRPE, 0), roadmap.MaxTargetRPE)
	}
	if !main {
		return reasons
	}

	band, banded := roadmap.PhaseRPEBand(sp.phase)
	banded = banded && sp.SessionKind != kindRecovery
	if banded {
		band = band.Capped(float32(injuryCap))
	}

	if ex.TargetRPE == 0 {
		reasons = append(reasons, "target_rpe is required on a main exercise")
		switch {
		case sp.SessionKind == kindRecovery:
			ex.TargetRPE = roadmap.RecoveryMaxRPE - 1
		case banded:
			ex.TargetRPE = float64(band.Min+band.Max) / 2
		default:
			ex.TargetRPE = defaultMainRPE
		}
		if injuryCap > 0 {
			ex.TargetRPE = min(ex.TargetRPE, injuryCap)
		}
		return reasons
	}

	if banded && !band.Contains(float32(ex.TargetRPE)) {
		reasons = append(reasons, fmt.Sprintf("target_rpe %.1f is outside the %s window %.1f to %.1f",
			ex.TargetRPE, sp.phase, band.Min, band.Max))
		ex.TargetRPE = float64(band.Clamp(float32(ex.TargetRPE)))
	}
	return reasons
}

// clampLoad bounds target_weight_kg: never negative, at most 30% above the
// athlete's record for the exercise (BR-AC-02), at most roadmap.UnrecordedLoadFactor ×
// bodyweight without a record, and never above the absolute ceiling. A zero
// load (bodyweight or timed work) is always fine.
func (v *planValidator) clampLoad(ex *PrescribedExercise) string {
	w := ex.TargetWeightKg
	switch {
	case w < 0:
		ex.TargetWeightKg = 0
		return fmt.Sprintf("target_weight_kg %.1f must not be negative", w)
	case w == 0:
		return ""
	}

	// Every cap applies and the lowest wins: the athlete's record or
	// bodyweight, and above both the absolute ceiling the guardrail enforces.
	var reason string
	if v.loads != nil {
		if baseline := v.loads.baselines[ex.ExerciseID]; baseline > 0 {
			if ceiling := service.NewOverloadValidator().Ceiling(baseline); w > ceiling {
				ex.TargetWeightKg = ceiling
				reason = fmt.Sprintf("target_weight_kg %.1f is more than 30%% above the athlete's record %.1f kg (at most %.1f)",
					w, baseline, ceiling)
			}
		} else if bw := v.loads.bodyWeightKg; bw > 0 && w > bw*roadmap.UnrecordedLoadFactor {
			ex.TargetWeightKg = bw * roadmap.UnrecordedLoadFactor
			reason = fmt.Sprintf("target_weight_kg %.1f exceeds %.1f kg (%.1f× bodyweight) for an exercise the athlete has no record of",
				w, ex.TargetWeightKg, roadmap.UnrecordedLoadFactor)
		}
	}
	if ex.TargetWeightKg > roadmap.MaxTargetWeightKg {
		ex.TargetWeightKg = roadmap.MaxTargetWeightKg
		reason = fmt.Sprintf("target_weight_kg %.1f exceeds the %d kg ceiling", w, roadmap.MaxTargetWeightKg)
	}
	return reason
}

const (
	// Timed lines without a duration get these in salvage: a warm-up or
	// cool-down hold, or a main-work interval.
	salvageStretchSeconds  = 30
	salvageIntervalSeconds = 45
	// salvageReps fills a repetition line without a count.
	salvageReps = 10
	// secondsPerRep converts reps the model gave a timed exercise into a
	// duration, bounded to a sensible hold.
	secondsPerRep    = 3
	minConvertedHold = 30
	maxConvertedHold = 60
)

// clampMeasure makes a line prescribe what its exercise is measured in. A
// timed exercise (plank, stretch, cardio interval) needs duration_seconds and
// carries no reps or load; a repetition exercise needs target_reps and no
// duration. A missing required field is reported; salvage fills it, turning
// reps given to a timed exercise into a hold of about three seconds each.
func clampMeasure(ex *PrescribedExercise, main bool, measure string) []string {
	var reasons []string
	if measure == measurementDuration {
		if ex.DurationSeconds <= 0 {
			reasons = append(reasons, "is timed: set duration_seconds (target_reps does not apply)")
			switch {
			case ex.TargetReps > 0:
				ex.DurationSeconds = min(max(ex.TargetReps*secondsPerRep, minConvertedHold), maxConvertedHold)
			case main:
				ex.DurationSeconds = salvageIntervalSeconds
			default:
				ex.DurationSeconds = salvageStretchSeconds
			}
		}
		dropStrayMeasureFields(ex, measure)
		return reasons
	}

	if ex.TargetReps <= 0 {
		reasons = append(reasons, "counts repetitions: set target_reps")
		ex.TargetReps = salvageReps
	}
	dropStrayMeasureFields(ex, measure)
	return reasons
}

// dropStrayMeasureFields clears what does not apply to the exercise's
// measurement type: reps and load on a timed exercise, a duration on a
// repetition one. Nothing is lost, so the model is not asked to fix it. Reps
// on a timed line still missing its duration are kept for clampMeasure to
// convert into a hold.
func dropStrayMeasureFields(ex *PrescribedExercise, measure string) {
	if measure == measurementDuration {
		if ex.DurationSeconds <= 0 {
			return
		}
		ex.TargetReps = 0
		ex.TargetWeightKg = 0
		return
	}
	ex.DurationSeconds = 0
}
