package adk

import (
	"fmt"
	"math"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

const (
	// rpeLoadDropPerPoint is how much lighter a load gets for each RPE point
	// the validator takes off: about 4% of 1RM per point at working rep ranges.
	rpeLoadDropPerPoint = 0.04

	// maxFittedShare is the share of main lines the validator may refit before
	// the plan goes back to the model: past it the model misread the task, and
	// rewriting the numbers alone would hide that. Up to freeFittedLines are
	// always fitted, so one slip in a two-line ad-hoc session costs no retry.
	maxFittedShare  = 0.3
	freeFittedLines = 2
)

// fitRPE moves a line's target_rpe into its window: the phase window for the
// main work of a TRAINING session, lowered by the exercise's injury cap. The
// window is the backend's to know, so a plan whose only defect is RPE ships
// without a retry. A lowered RPE lowers the load with it: a load picked for
// RPE 8.5 is too heavy at RPE 7. It reports what it changed, "" when nothing.
// A missing or off-scale RPE is clampRPE's to report.
func fitRPE(ex *PrescribedExercise, main bool, sp *SessionPlan, injuryCap float64) string {
	if ex.TargetRPE <= 0 || ex.TargetRPE > roadmap.MaxTargetRPE {
		return ""
	}

	lo, hi := 0.0, float64(roadmap.MaxTargetRPE)
	band, banded := roadmap.PhaseRPEBand(sp.phase)
	banded = banded && main && sp.SessionKind != kindRecovery
	if banded {
		band = band.Capped(float32(injuryCap))
		lo, hi = float64(band.Min), float64(band.Max)
	} else if injuryCap > 0 {
		hi = injuryCap
	}

	before := ex.TargetRPE
	after := min(max(before, lo), hi)
	if after == before {
		return ""
	}
	ex.TargetRPE = after

	var reason string
	if injuryCap > 0 && before > injuryCap && after == injuryCap {
		reason = fmt.Sprintf("target_rpe %.1f exceeds the injury cap %s; set to %.1f",
			before, maxRPEText(injuryCap), after)
	} else {
		reason = fmt.Sprintf("target_rpe %.1f is outside the %s window %.1f to %.1f; set to %.1f",
			before, sp.phase, lo, hi, after)
	}

	if after < before && ex.TargetWeightKg > 0 {
		w := ex.TargetWeightKg
		ex.TargetWeightKg = math.Round(w*(1-rpeLoadDropPerPoint*(before-after))*2) / 2
		reason += fmt.Sprintf(", target_weight_kg %.1f to %.1f", w, ex.TargetWeightKg)
	}
	return reason
}

// fitRecordLoad lowers a load above BR-AC-02's ceiling (30% over the
// athlete's record for the exercise) to that ceiling, like fitRPE: the record
// is the backend's to know, and a retry to move 20 kg to 19.5 kg costs a model
// call. Loads without a record, and the absolute ceiling, stay clampLoad's.
func (v *planValidator) fitRecordLoad(ex *PrescribedExercise) string {
	if v.loads == nil || ex.TargetWeightKg <= 0 {
		return ""
	}
	baseline := v.loads.baselines[ex.ExerciseID]
	if baseline <= 0 {
		return ""
	}
	ceiling := service.NewOverloadValidator().Ceiling(baseline)
	if ex.TargetWeightKg <= ceiling {
		return ""
	}
	w := ex.TargetWeightKg
	ex.TargetWeightKg = math.Floor(ceiling*2) / 2
	return fmt.Sprintf("target_weight_kg %.1f is more than 30%% above the athlete's record %.1f kg; set to %.1f",
		w, baseline, ex.TargetWeightKg)
}

// splitFitted separates what the validator fixed itself from what the model
// must fix. A plan with too many fitted lines goes back whole: the fitted
// lines rejoin the issues, so the model hears what it got wrong.
func splitFitted(plan *GeneratedPlan, issues []planIssue, final bool) (open, fitted []planIssue) {
	for _, is := range issues {
		if is.Fitted {
			fitted = append(fitted, is)
		} else {
			open = append(open, is)
		}
	}
	if !final && len(fitted) > freeFittedLines &&
		float64(len(fitted)) > maxFittedShare*float64(mainLineCount(plan)) {
		return append(open, fitted...), nil
	}
	return open, fitted
}

// mainLineCount counts the main-exercise lines of a plan.
func mainLineCount(plan *GeneratedPlan) int {
	n := 0
	for _, w := range plan.Weeks {
		for i := range w.Sessions {
			n += len(w.Sessions[i].Prescription.MainExercises)
		}
	}
	return n
}
