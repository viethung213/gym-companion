package adk

import (
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

const (
	tagMobilityWarmUp     = "Mobility & Warm-up"
	tagStretchingCoolDown = "Stretching & Cool-down"

	defaultMuscleQuota   = 3
	preferredMuscleQuota = 8
)

// exerciseMetaResolver abstracts metadata ID->Name lookups for pure selector functions.
type exerciseMetaResolver struct {
	muscle    func(idOrName string) string
	bodyPart  func(idOrName string) string
	equipment func(idOrName string) string
	tag       func(idOrName string) string
}

func (r exerciseMetaResolver) resolveMuscle(s string) string {
	if r.muscle != nil {
		return r.muscle(s)
	}
	return s
}

func (r exerciseMetaResolver) resolveBodyPart(s string) string {
	if r.bodyPart != nil {
		return r.bodyPart(s)
	}
	return s
}

func (r exerciseMetaResolver) resolveEquipment(s string) string {
	if r.equipment != nil {
		return r.equipment(s)
	}
	return s
}

func (r exerciseMetaResolver) resolveTag(s string) string {
	if r.tag != nil {
		return r.tag(s)
	}
	return s
}

func hasExerciseTag(ex *port.Exercise, targetTag string, r exerciseMetaResolver) bool {
	for _, tid := range ex.TagIDs {
		if strings.EqualFold(tid, targetTag) || strings.EqualFold(r.resolveTag(tid), targetTag) {
			return true
		}
	}
	return false
}

func isBodyweightExercise(ex *port.Exercise, eqResolved string) bool {
	return ex.IsBodyweight ||
		strings.EqualFold(ex.Equipment, "Body Weight") ||
		strings.EqualFold(eqResolved, "Body Weight") ||
		strings.EqualFold(eqResolved, "bodyweight") ||
		ex.Equipment == ""
}

func isCardioExercise(ex *port.Exercise, exMuscleResolved, exBodyPartResolved string) bool {
	return strings.EqualFold(ex.MuscleGroup, "Cardio") ||
		strings.EqualFold(exMuscleResolved, "Cardiovascular System") ||
		strings.EqualFold(exBodyPartResolved, "Cardio")
}

func buildExerciseTuple(ex *port.Exercise, eqResolved, movementType string) []string {
	measType := ex.MeasurementType
	if measType == "" {
		measType = "REPS"
	}
	return []string{
		ex.ExerciseID,
		ex.Name,
		eqResolved,
		ex.Difficulty,
		movementType,
		measType,
	}
}

// filterEligibleExercises applies all hard safety & capability filters:
// 1. Exclude active injuries (matching both MuscleGroup and BodyPart).
// 2. Enforce exercise_difficulty <= user_experience_level.
// 3. Match user's available equipment (+ Body Weight).
func filterEligibleExercises(
	exercises []port.Exercise,
	profile *port.Profile,
	avoidInjuries []string,
	r exerciseMetaResolver,
) []port.Exercise {
	eligible := make([]port.Exercise, 0, len(exercises))
	for i := range exercises {
		ex := &exercises[i]
		exMuscleResolved := r.resolveMuscle(ex.MuscleGroup)
		exBodyPartResolved := r.resolveBodyPart(ex.BodyPart)

		conflict := false
		for _, inj := range avoidInjuries {
			injResolved := r.resolveMuscle(inj)
			if strings.EqualFold(ex.MuscleGroup, inj) ||
				strings.EqualFold(exMuscleResolved, injResolved) ||
				(ex.BodyPart != "" && (strings.EqualFold(ex.BodyPart, inj) || strings.EqualFold(exBodyPartResolved, injResolved))) {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}

		if !isDifficultyAllowed(ex.Difficulty, profile.ExperienceLevel) {
			continue
		}

		eqResolved := r.resolveEquipment(ex.Equipment)
		if len(profile.AvailableEquipment) > 0 && !isBodyweightExercise(ex, eqResolved) {
			eqMatch := false
			for _, eq := range profile.AvailableEquipment {
				userEqResolved := r.resolveEquipment(eq)
				if strings.EqualFold(ex.Equipment, eq) || strings.EqualFold(eqResolved, userEqResolved) {
					eqMatch = true
					break
				}
			}
			if !eqMatch {
				continue
			}
		}

		eligible = append(eligible, *ex)
	}
	return eligible
}

// selectMainCandidates builds the main_exercises map dynamically from the DB's eligible exercises.
// User-preferred muscles (matched by MuscleGroup or BodyPart) receive preferredMuscleQuota (8);
// all other muscles in the catalog receive defaultMuscleQuota (3) for balanced programming.
// Cardio is included when the user explicitly prefers it or when PrimaryGoal targets fat loss / endurance.
func selectMainCandidates(
	eligible []port.Exercise,
	profile *port.Profile,
	r exerciseMetaResolver,
) (candidates map[string][][]string, activeBodyParts map[string]bool) {
	candidates = make(map[string][][]string)
	activeBodyParts = make(map[string]bool)

	goalLower := strings.ToLower(profile.PrimaryGoal)
	goalWantsCardio := strings.Contains(goalLower, "fat") || strings.Contains(goalLower, "endurance")

	isPreferred := func(ex *port.Exercise, exMuscleResolved, exBodyPartResolved string) bool {
		for _, mg := range profile.PreferredMuscleGroups {
			mgResolved := r.resolveMuscle(mg)
			if strings.EqualFold(ex.MuscleGroup, mg) ||
				strings.EqualFold(exMuscleResolved, mgResolved) ||
				(ex.BodyPart != "" && (strings.EqualFold(ex.BodyPart, mg) || strings.EqualFold(exBodyPartResolved, mgResolved))) {
				return true
			}
		}
		return false
	}

	for i := range eligible {
		ex := &eligible[i]
		if hasExerciseTag(ex, tagStretchingCoolDown, r) || hasExerciseTag(ex, tagMobilityWarmUp, r) {
			continue
		}

		exMuscleResolved := r.resolveMuscle(ex.MuscleGroup)
		exBodyPartResolved := r.resolveBodyPart(ex.BodyPart)
		preferred := isPreferred(ex, exMuscleResolved, exBodyPartResolved)

		if isCardioExercise(ex, exMuscleResolved, exBodyPartResolved) && !goalWantsCardio && !preferred {
			continue
		}

		quota := defaultMuscleQuota
		if preferred {
			quota = preferredMuscleQuota
		}

		group := exMuscleResolved
		if group == "" {
			group = "General"
		}
		if len(candidates[group]) >= quota {
			continue
		}

		eqResolved := r.resolveEquipment(ex.Equipment)
		measType := ex.MeasurementType
		if measType == "" {
			measType = "REPS"
		}
		movementType := "Compound"
		if ex.IsMachineOrCable {
			movementType = "Accessory"
		}
		if isCardioExercise(ex, exMuscleResolved, exBodyPartResolved) || measType == "DURATION" {
			movementType = "Cardio"
		}

		candidates[group] = append(candidates[group], buildExerciseTuple(ex, eqResolved, movementType))
		if exBodyPartResolved != "" {
			activeBodyParts[exBodyPartResolved] = true
		}
	}

	return candidates, activeBodyParts
}

// selectRecoveryCandidates builds dedicated warm_up_exercises and cool_down_exercises
// maps grouped by body part (+ "General"), strictly disjoint from main_exercises.
func selectRecoveryCandidates(
	eligible []port.Exercise,
	activeBodyParts map[string]bool,
	r exerciseMetaResolver,
) (warmUps, coolDowns map[string][][]string) {
	warmUps = make(map[string][][]string)
	coolDowns = make(map[string][][]string)

	warmSeen := make(map[string]bool)
	coolSeen := make(map[string]bool)

	// 1. Warm-ups: strictly exercises tagged Mobility & Warm-up.
	for i := range eligible {
		ex := &eligible[i]
		if !hasExerciseTag(ex, tagMobilityWarmUp, r) {
			continue
		}
		bp := r.resolveBodyPart(ex.BodyPart)
		bucket := "General"
		capLimit := 6
		if bp != "" && activeBodyParts[bp] {
			bucket = bp
			capLimit = 3
		}
		if len(warmUps[bucket]) >= capLimit || warmSeen[ex.ExerciseID] {
			continue
		}
		eqResolved := r.resolveEquipment(ex.Equipment)
		warmUps[bucket] = append(warmUps[bucket], buildExerciseTuple(ex, eqResolved, "Mobility"))
		warmSeen[ex.ExerciseID] = true
	}

	// 2. Cool-downs: strictly exercises tagged Stretching & Cool-down matching activeBodyParts first.
	for i := range eligible {
		ex := &eligible[i]
		if !hasExerciseTag(ex, tagStretchingCoolDown, r) {
			continue
		}
		bp := r.resolveBodyPart(ex.BodyPart)
		if bp != "" && activeBodyParts[bp] {
			if len(coolDowns[bp]) < 3 && !coolSeen[ex.ExerciseID] {
				eqResolved := r.resolveEquipment(ex.Equipment)
				coolDowns[bp] = append(coolDowns[bp], buildExerciseTuple(ex, eqResolved, "Stretch"))
				coolSeen[ex.ExerciseID] = true
			}
		}
	}

	// Fill "General" cool-down stretches for broader recovery options.
	for i := range eligible {
		if len(coolDowns["General"]) >= 4 {
			break
		}
		ex := &eligible[i]
		if !hasExerciseTag(ex, tagStretchingCoolDown, r) || coolSeen[ex.ExerciseID] {
			continue
		}
		eqResolved := r.resolveEquipment(ex.Equipment)
		coolDowns["General"] = append(coolDowns["General"], buildExerciseTuple(ex, eqResolved, "Stretch"))
		coolSeen[ex.ExerciseID] = true
	}

	return warmUps, coolDowns
}
