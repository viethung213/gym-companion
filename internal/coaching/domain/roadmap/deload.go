package roadmap

import (
	"fmt"
	"time"
)

// IsTrainingVolume reports whether a session's main work counts as training
// volume for BR-AC-11: scheduled TRAINING sessions only. Recovery work is not
// training, and an ad-hoc or solo session is the athlete's own pick.
func (s *SessionPlan) IsTrainingVolume() bool {
	return s.Kind() != SessionKindRecovery && s.info.Source == SessionPlanSourceScheduled
}

// mainSets totals the sets of the session's main exercises.
func (s *SessionPlan) mainSets() int {
	total := 0
	for _, ex := range s.info.Prescription.MainExercises {
		total += int(ex.TargetSets)
	}
	return total
}

// WorkingSets totals the main-exercise sets of the training-volume sessions
// in the weeks of the phase.
func (r *Roadmap) WorkingSets(phase Phase) int {
	total := 0
	r.eachTrainingSession(phase, func(s *SessionPlan) { total += s.mainSets() })
	return total
}

// DeloadAllowance is the most main working sets the DELOAD week may carry:
// DeloadVolumeRatio of the PEAK week's (BR-AC-11). ok is false when the
// roadmap has no PEAK volume to unload from.
func (r *Roadmap) DeloadAllowance() (allowed int, ok bool) {
	peak := r.WorkingSets(PhasePeak)
	if peak == 0 {
		return 0, false
	}
	return int(float64(peak) * DeloadVolumeRatio), true
}

// FitDeloadVolume enforces BR-AC-11 on the sessions still to come. When the
// DELOAD week carries more than its allowance, sets are removed from its
// upcoming PENDING training sessions, one at a time from the line with the
// most sets, never below one set per line. Finished and started sessions are
// left as they are; if they alone exceed the allowance, the upcoming ones go
// down to one set per line. It reports whether any session changed.
func (r *Roadmap) FitDeloadVolume(now time.Time) (bool, error) {
	allowed, ok := r.DeloadAllowance()
	if !ok || r.WorkingSets(PhaseDeload) <= allowed {
		return false, nil
	}

	upcoming := make(map[string]bool)
	for _, s := range r.UpcomingPendingSessions(now) {
		upcoming[s.ID()] = true
	}

	// Copies of the upcoming DELOAD prescriptions, cut down together.
	var sessions []*SessionPlan
	var prescriptions []WorkoutPrescription
	excess := r.WorkingSets(PhaseDeload) - allowed
	r.eachTrainingSession(PhaseDeload, func(s *SessionPlan) {
		if upcoming[s.ID()] {
			sessions = append(sessions, s)
			prescriptions = append(prescriptions, copyPrescription(s.info.Prescription))
		}
	})

	cut := trimMainSets(prescriptions, excess)

	changed := false
	for i, s := range sessions {
		if !cut[i] {
			continue
		}
		info := s.Info()
		if err := s.RewritePrescription(prescriptions[i], info.TargetMuscleGroups, info.Reasoning, now); err != nil {
			return changed, fmt.Errorf("fit DELOAD volume of session %s: %w", s.ID(), err)
		}
		changed = true
	}
	return changed, nil
}

// trimMainSets removes excess main sets from the prescriptions, one at a
// time from the line with the most sets, never below one set per line. It
// reports which prescriptions lost a set.
func trimMainSets(prescriptions []WorkoutPrescription, excess int) []bool {
	cut := make([]bool, len(prescriptions))
	for ; excess > 0; excess-- {
		si, ei := -1, -1
		for i := range prescriptions {
			for j, ex := range prescriptions[i].MainExercises {
				if ex.TargetSets > 1 && (si < 0 || ex.TargetSets > prescriptions[si].MainExercises[ei].TargetSets) {
					si, ei = i, j
				}
			}
		}
		if si < 0 {
			break // every line is at one set
		}
		prescriptions[si].MainExercises[ei].TargetSets--
		cut[si] = true
	}
	return cut
}

// MainSets totals the sets of the session's main exercises.
func (s *SessionPlan) MainSets() int { return s.mainSets() }

// UnloadUpcoming enforces a fatigue deload's volume cut (BR-AC-04 case 2) on
// the upcoming PENDING training sessions: each keeps at most ratio of the
// main sets it had before the adaptation (before, by session ID), cut from
// its largest lines, never below one set per line. Sessions of a DELOAD week
// are skipped: BR-AC-11 already unloads them, and cutting again would leave
// barely a workout. Sessions missing from before are left alone. It returns
// the IDs of the sessions it cut.
func (r *Roadmap) UnloadUpcoming(before map[string]int, ratio float64, now time.Time) ([]string, error) {
	upcoming := make(map[string]bool)
	for _, s := range r.UpcomingPendingSessions(now) {
		upcoming[s.ID()] = true
	}

	var cutIDs []string
	for _, w := range r.weeks {
		if w.Phase() == PhaseDeload {
			continue
		}
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				had, known := before[s.ID()]
				if !s.IsTrainingVolume() || !upcoming[s.ID()] || !known {
					continue
				}
				excess := s.mainSets() - int(float64(had)*ratio)
				if excess <= 0 {
					continue
				}
				p := []WorkoutPrescription{copyPrescription(s.info.Prescription)}
				if !trimMainSets(p, excess)[0] {
					continue
				}
				info := s.Info()
				if err := s.RewritePrescription(p[0], info.TargetMuscleGroups, info.Reasoning, now); err != nil {
					return cutIDs, fmt.Errorf("unload session %s: %w", s.ID(), err)
				}
				cutIDs = append(cutIDs, s.ID())
			}
		}
	}
	return cutIDs, nil
}

func (r *Roadmap) eachTrainingSession(phase Phase, visit func(*SessionPlan)) {
	for _, w := range r.weeks {
		if w.Phase() != phase {
			continue
		}
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if s.IsTrainingVolume() {
					visit(s)
				}
			}
		}
	}
}
