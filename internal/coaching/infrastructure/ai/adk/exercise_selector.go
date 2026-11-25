package adk

import (
	"sort"
	"strconv"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

const (
	defaultMuscleQuota   = 3
	preferredMuscleQuota = 8

	generalBucket         = "General"
	recoveryPerMuscleCap  = 3
	recoveryGeneralCap    = 6
	movementTypeCompound  = "Compound"
	movementTypeAccessory = "Accessory"
	movementTypeCardio    = "Cardio"
	movementTypeStretch   = "Stretch"
	measurementReps       = "REPS"
	measurementDuration   = "DURATION"
)

// candidate is an exercise that passed the hard filters, with its injury verdict.
type candidate struct {
	exercise port.Exercise
	traits   service.ExerciseTraits
	verdict  service.Verdict
}

// candidatePrefs steers which main-work muscles get the larger quota.
type candidatePrefs struct {
	PreferredMuscles []taxonomy.MuscleGroup
	PrimaryGoal      string
}

// candidateLists is what the generator may pick from.
type candidateLists struct {
	main     map[string][][]string // primary MuscleGroup code -> tuples
	recovery map[string][][]string // primary MuscleGroup code or "General" -> tuples
	limits   map[string]string     // exercise_id -> injury limit
}

// buildCandidateLists filters the catalog for the athlete and groups what is
// left into main work and recovery (warm-up / cool-down) candidates.
func buildCandidateLists(exercises []port.Exercise, prefs *candidatePrefs, athlete *service.Athlete) candidateLists {
	eligible := filterEligibleExercises(exercises, athlete)
	lists := candidateLists{limits: make(map[string]string)}

	var activeMuscles map[string]bool
	lists.main, activeMuscles = selectMainCandidates(eligible, prefs, lists.limits)
	lists.recovery = selectRecoveryCandidates(eligible, activeMuscles, lists.limits)
	return lists
}

// filterEligibleExercises applies the hard filters of docs/05 §8.2-8.4: level
// <= the athlete's, equipment owned (BODY_ONLY always) and not excluded by an
// injury. Where an injury prefers machine or cable, guided exercises move
// ahead of the others, otherwise catalog order is kept.
func filterEligibleExercises(exercises []port.Exercise, athlete *service.Athlete) []candidate {
	eligible := make([]candidate, 0, len(exercises))
	for i := range exercises {
		c := candidate{exercise: exercises[i], traits: exercises[i].Traits()}
		if !service.LevelAllowed(c.traits.Level, athlete.Level) ||
			!service.EquipmentAvailable(c.traits.Equipment, athlete.Equipment) {
			continue
		}
		c.verdict = service.EvaluateInjuries(&c.traits, athlete.Injuries)
		if c.verdict.Excluded {
			continue
		}
		eligible = append(eligible, c)
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		return guidedRank(&eligible[i]) < guidedRank(&eligible[j])
	})
	return eligible
}

// guidedRank is 1 for a non-guided exercise an injury would rather replace.
func guidedRank(c *candidate) int {
	if c.verdict.PreferMachineOrCable && !c.traits.IsMachineOrCable() {
		return 1
	}
	return 0
}

// selectMainCandidates groups main work (every category but STRETCHING) by
// primary muscle. Preferred muscles get preferredMuscleQuota, the rest
// defaultMuscleQuota. Cardio is kept only when preferred or when the goal
// targets fat loss or endurance.
func selectMainCandidates(
	eligible []candidate,
	prefs *candidatePrefs,
	limits map[string]string,
) (candidates map[string][][]string, activeMuscles map[string]bool) {
	candidates = make(map[string][][]string)
	activeMuscles = make(map[string]bool)

	goalLower := strings.ToLower(prefs.PrimaryGoal)
	goalWantsCardio := strings.Contains(goalLower, "fat") || strings.Contains(goalLower, "endurance")
	preferred := make(map[taxonomy.MuscleGroup]bool, len(prefs.PreferredMuscles))
	for _, m := range prefs.PreferredMuscles {
		preferred[m] = true
	}

	for i := range eligible {
		c := &eligible[i]
		if c.traits.IsWarmUpCoolDown() {
			continue
		}
		isPreferred := preferred[c.traits.PrimaryMuscle]
		isCardio := c.traits.Category == taxonomy.CategoryCardio
		if isCardio && !goalWantsCardio && !isPreferred {
			continue
		}

		quota := defaultMuscleQuota
		if isPreferred {
			quota = preferredMuscleQuota
		}
		group := string(c.traits.PrimaryMuscle)
		if len(candidates[group]) >= quota {
			continue
		}

		candidates[group] = append(candidates[group], buildExerciseTuple(c, mainMovementType(c)))
		activeMuscles[group] = true
		recordLimit(c, limits)
	}

	return candidates, activeMuscles
}

// selectRecoveryCandidates picks STRETCHING exercises for warm-ups and
// cool-downs (docs/05 §8.4), bucketed by primary muscle when that muscle is
// trained in main work and under "General" otherwise.
func selectRecoveryCandidates(
	eligible []candidate,
	activeMuscles map[string]bool,
	limits map[string]string,
) map[string][][]string {
	recovery := make(map[string][][]string)
	for i := range eligible {
		c := &eligible[i]
		if !c.traits.IsWarmUpCoolDown() {
			continue
		}

		bucket, capLimit := generalBucket, recoveryGeneralCap
		if muscle := string(c.traits.PrimaryMuscle); activeMuscles[muscle] {
			bucket, capLimit = muscle, recoveryPerMuscleCap
		}
		if len(recovery[bucket]) >= capLimit {
			continue
		}

		recovery[bucket] = append(recovery[bucket], buildExerciseTuple(c, movementTypeStretch))
		recordLimit(c, limits)
	}
	return recovery
}

// mainMovementType labels main work for the generator from source data.
func mainMovementType(c *candidate) string {
	switch {
	case c.traits.Category == taxonomy.CategoryCardio || c.exercise.MeasurementType == measurementDuration:
		return movementTypeCardio
	case c.traits.Mechanic == taxonomy.MechanicCompound:
		return movementTypeCompound
	default:
		return movementTypeAccessory
	}
}

// recordLimit notes the injury limit of an exercise kept with an RPE cap.
func recordLimit(c *candidate, limits map[string]string) {
	if c.verdict.MaxRPE > 0 {
		limits[c.exercise.ExerciseID] = injuryLimitText(&c.verdict)
	}
}

// injuryLimitText is the limit shown to the generator and kept in reasoning.
func injuryLimitText(v *service.Verdict) string {
	return maxRPEText(v.MaxRPE) + " (" + v.Reason() + ")"
}

func maxRPEText(maxRPE float64) string {
	return "max RPE " + strconv.FormatFloat(maxRPE, 'f', -1, 64)
}

// buildExerciseTuple is [id, name, equipment, level, movement type,
// measurement], plus the max RPE of an exercise kept despite an injury, so the
// cap sits beside the exercise the model picks.
func buildExerciseTuple(c *candidate, movementType string) []string {
	measType := c.exercise.MeasurementType
	if measType == "" {
		measType = measurementReps
	}
	tuple := []string{
		c.exercise.ExerciseID,
		c.exercise.Name,
		string(c.traits.Equipment),
		string(c.traits.Level),
		movementType,
		measType,
	}
	if c.verdict.MaxRPE > 0 {
		tuple = append(tuple, maxRPEText(c.verdict.MaxRPE))
	}
	return tuple
}
