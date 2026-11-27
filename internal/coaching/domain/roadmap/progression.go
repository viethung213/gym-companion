package roadmap

import (
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// Progressive overload (BR-AC-04 case 1) is applied by these rules, not left
// to the model, which answered "+2.5%" with anything from +0% to +33%. A
// loaded line uses double progression: it gains reps inside the goal's range,
// and at the top of the range it takes one real load step (a dumbbell or a
// plate pair) and drops back to the bottom of the range. Bodyweight work gains
// reps, holds gain seconds, each up to a ceiling.
const (
	maxProgressedReps    = 20 // bodyweight reps past this are endurance work
	maxProgressedHoldSec = 90
	holdStepSec          = 5
)

// RepRange is the repetitions a loaded line climbs through.
type RepRange struct{ Min, Max int32 }

// RepRangeForGoal is the rep range of the athlete's primary goal; an unknown
// goal trains for hypertrophy, the default goal.
func RepRangeForGoal(goal string) RepRange {
	switch strings.ToUpper(strings.TrimSpace(goal)) {
	case "STRENGTH":
		return RepRange{4, 6}
	case "ENDURANCE":
		return RepRange{12, 15}
	case "LOSE_FAT":
		return RepRange{10, 15}
	default: // BUILD_MUSCLE, hypertrophy
		return RepRange{8, 12}
	}
}

// LoadStep is the smallest real load increase for the equipment: a dumbbell
// or kettlebell size, a pair of 1.25 kg plates, a weight-stack pin. 0 when the
// load cannot be stepped (bands, bodyweight, balls).
func LoadStep(e taxonomy.Equipment) float32 {
	switch e {
	case taxonomy.EquipmentDumbbell, taxonomy.EquipmentKettlebells:
		return 2
	case taxonomy.EquipmentBarbell, taxonomy.EquipmentEZCurlBar, taxonomy.EquipmentMachine, taxonomy.EquipmentCable:
		return 2.5
	default:
		return 0
	}
}

// ProgressLine progresses one main-work line. loadStep is LoadStep of the
// exercise's equipment; ceilingKg (0 = none) is the most the athlete may lift
// (BR-AC-02): a step past it leaves the line as it was.
func ProgressLine(from *PrescribedExercise, loadStep float32, rng RepRange, ceilingKg float32) PrescribedExercise {
	line := *from
	switch {
	case line.DurationSeconds > 0 && line.TargetReps == 0:
		line.DurationSeconds = min(line.DurationSeconds+holdStepSec, max(line.DurationSeconds, maxProgressedHoldSec))
	case line.TargetWeight <= 0:
		line.TargetReps = min(line.TargetReps+2, max(line.TargetReps, maxProgressedReps))
	case line.TargetReps < rng.Max:
		line.TargetReps = min(line.TargetReps+repStep(rng), rng.Max)
	case loadStep > 0:
		next := line.TargetWeight + loadStep
		if ceilingKg > 0 && next > ceilingKg {
			return line
		}
		line.TargetWeight, line.TargetReps = next, rng.Min
	}
	return line
}

// repStep is how many reps a loaded line gains a week: 2, or 1 in a narrow
// (strength) range where 2 would skip half of it.
func repStep(rng RepRange) int32 {
	if rng.Max-rng.Min <= 2 {
		return 1
	}
	return 2
}

// LineProgression is how one exercise progresses: its LoadStep and the most
// the athlete may lift (0 = no record to bound it).
type LineProgression struct {
	LoadStep  float32
	CeilingKg float32
}

// ProgressUpcoming applies progressive overload to the upcoming PENDING
// training sessions, from the main lines they held before the adaptation
// (before, by session ID): the model is asked to keep the numbers, and these
// rules set them. A line progresses when its exercise was in the session
// before and has a rule; an exercise the model swapped in, or one the caller
// left without a rule (an injury limits it), keeps the model's numbers. The
// DELOAD week is not progressed. It returns the IDs of the sessions changed.
func (r *Roadmap) ProgressUpcoming(
	before map[string][]PrescribedExercise, rng RepRange, rules map[string]LineProgression, now time.Time,
) ([]string, error) {
	upcoming := make(map[string]bool)
	for _, s := range r.UpcomingPendingSessions(now) {
		upcoming[s.ID()] = true
	}

	var changed []string
	for _, w := range r.weeks {
		if w.Phase() == PhaseDeload {
			continue
		}
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				prev, known := before[s.ID()]
				if !s.IsTrainingVolume() || !upcoming[s.ID()] || !known {
					continue
				}
				p := copyPrescription(s.info.Prescription)
				if !progressLines(p.MainExercises, prev, rng, rules) {
					continue
				}
				info := s.Info()
				if err := s.RewritePrescription(p, info.TargetMuscleGroups, info.Reasoning, now); err != nil {
					return changed, fmt.Errorf("progress session %s: %w", s.ID(), err)
				}
				changed = append(changed, s.ID())
			}
		}
	}
	return changed, nil
}

// progressLines sets each line's numbers from its exercise's earlier line,
// progressed, keeping the line's own RPE, rest and notes. It reports whether
// any line changed.
func progressLines(lines, prev []PrescribedExercise, rng RepRange, rules map[string]LineProgression) bool {
	// An exercise may appear on several lines: they pair up in order.
	earlier := make(map[string][]PrescribedExercise, len(prev))
	for _, e := range prev {
		earlier[e.ExerciseID] = append(earlier[e.ExerciseID], e)
	}
	changed := false
	for i := range lines {
		id := lines[i].ExerciseID
		rule, ok := rules[id]
		if !ok || len(earlier[id]) == 0 {
			continue // no rule (injury-limited), or swapped in by the model
		}
		from := earlier[id][0]
		earlier[id] = earlier[id][1:]
		p := ProgressLine(&from, rule.LoadStep, rng, rule.CeilingKg)
		updated := lines[i]
		updated.TargetSets, updated.TargetReps = p.TargetSets, p.TargetReps
		updated.TargetWeight, updated.DurationSeconds = p.TargetWeight, p.DurationSeconds
		if updated != lines[i] {
			lines[i], changed = updated, true
		}
	}
	return changed
}
