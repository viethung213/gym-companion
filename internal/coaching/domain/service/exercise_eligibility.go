package service

import (
	"sort"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// Exercise eligibility rules of docs/05_exercise_catalog_refactor_spec.md §8,
// shared by roadmap selection, ad-hoc sessions, the guardrail and the
// replacement tool. They read only free-exercise-db data plus the region to
// muscle table.

const (
	// moderateSecondaryMaxRPE caps exercises that load a MODERATE injury as secondary.
	moderateSecondaryMaxRPE = 6.0
	// mildMaxRPE caps exercises that load a MILD injury.
	mildMaxRPE = 7.0
)

// ExerciseTraits is the source data the rules read.
type ExerciseTraits struct {
	PrimaryMuscle    taxonomy.MuscleGroup
	SecondaryMuscles []taxonomy.MuscleGroup
	Equipment        taxonomy.Equipment
	Category         taxonomy.Category
	Level            taxonomy.Level
	Force            taxonomy.Force
	Mechanic         taxonomy.Mechanic
}

// IsBodyweight reports an exercise that needs no equipment.
func (t *ExerciseTraits) IsBodyweight() bool {
	return t.Equipment == taxonomy.EquipmentBodyOnly
}

// IsMachineOrCable reports a guided-path exercise, preferred around injuries.
func (t *ExerciseTraits) IsMachineOrCable() bool {
	return t.Equipment == taxonomy.EquipmentMachine || t.Equipment == taxonomy.EquipmentCable
}

// IsHighImpact approximates high impact by category: the source has no impact field (§8.1).
func (t *ExerciseTraits) IsHighImpact() bool {
	switch t.Category {
	case taxonomy.CategoryPlyometrics, taxonomy.CategoryOlympicWeightlifting, taxonomy.CategoryStrongman:
		return true
	default:
		return false
	}
}

// IsWarmUpCoolDown reports a stretching exercise, used for warm-ups and
// cool-downs; every other category is main work (§8.4).
func (t *ExerciseTraits) IsWarmUpCoolDown() bool {
	return t.Category == taxonomy.CategoryStretching
}

// RegionMuscles returns the muscle groups covered by a joint region (§4).
func RegionMuscles(region taxonomy.JointRegion) []taxonomy.MuscleGroup {
	switch region {
	case taxonomy.JointRegionNeck:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupNeck, taxonomy.MuscleGroupTraps}
	case taxonomy.JointRegionShoulder:
		return []taxonomy.MuscleGroup{
			taxonomy.MuscleGroupShoulders, taxonomy.MuscleGroupChest, taxonomy.MuscleGroupLats,
			taxonomy.MuscleGroupTraps, taxonomy.MuscleGroupBiceps, taxonomy.MuscleGroupTriceps,
		}
	case taxonomy.JointRegionElbow:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupBiceps, taxonomy.MuscleGroupTriceps, taxonomy.MuscleGroupForearms}
	case taxonomy.JointRegionWristHand:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupForearms}
	case taxonomy.JointRegionUpperBack:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupMiddleBack, taxonomy.MuscleGroupLats, taxonomy.MuscleGroupTraps}
	case taxonomy.JointRegionLowerBack:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupLowerBack, taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupAbdominals}
	case taxonomy.JointRegionChest:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupChest}
	case taxonomy.JointRegionHip:
		return []taxonomy.MuscleGroup{
			taxonomy.MuscleGroupGlutes, taxonomy.MuscleGroupQuadriceps, taxonomy.MuscleGroupHamstrings,
			taxonomy.MuscleGroupAdductors, taxonomy.MuscleGroupAbductors,
		}
	case taxonomy.JointRegionKnee:
		return []taxonomy.MuscleGroup{
			taxonomy.MuscleGroupQuadriceps, taxonomy.MuscleGroupHamstrings, taxonomy.MuscleGroupCalves,
			taxonomy.MuscleGroupAdductors, taxonomy.MuscleGroupAbductors,
		}
	case taxonomy.JointRegionAnkleFoot:
		return []taxonomy.MuscleGroup{taxonomy.MuscleGroupCalves}
	default:
		return nil
	}
}

// Injury is an active injury, or a temporary ad-hoc avoid (§8.5) that may name
// only a muscle.
type Injury struct {
	Region   taxonomy.JointRegion
	Muscle   taxonomy.MuscleGroup
	Severity taxonomy.InjurySeverity
}

// Muscles returns M(injury): the region's muscles plus the reported muscle (§8.1).
func (i *Injury) Muscles() map[taxonomy.MuscleGroup]bool {
	m := make(map[taxonomy.MuscleGroup]bool)
	for _, muscle := range RegionMuscles(i.Region) {
		m[muscle] = true
	}
	if i.Muscle != "" {
		m[i.Muscle] = true
	}
	return m
}

// Label names the injury for reasoning and violation messages.
func (i *Injury) Label() string {
	area := string(i.Region)
	switch {
	case area == "":
		area = string(i.Muscle)
	case i.Muscle != "":
		area += "/" + string(i.Muscle)
	}
	return area + " (" + string(i.Severity) + ")"
}

// Verdict is the strictest outcome of every injury for one exercise.
type Verdict struct {
	Excluded bool
	// MaxRPE caps the prescribed RPE when > 0.
	MaxRPE               float64
	PreferMachineOrCable bool
	// Reasons explain each injury that affected the exercise.
	Reasons []string
}

// Reason joins the reasons into one sentence.
func (v *Verdict) Reason() string {
	return strings.Join(v.Reasons, "; ")
}

func (v *Verdict) exclude(reason string) {
	v.Excluded = true
	v.Reasons = append(v.Reasons, reason)
}

func (v *Verdict) limit(maxRPE float64, reason string) {
	if v.MaxRPE == 0 || maxRPE < v.MaxRPE {
		v.MaxRPE = maxRPE
	}
	v.Reasons = append(v.Reasons, reason)
}

// EvaluateInjuries applies the §8.2 table for each injury and keeps the
// strictest result:
//
//	SEVERE:   exclude primary or secondary load.
//	MODERATE: exclude primary load, and secondary load on a high-impact exercise;
//	          other secondary load is kept at RPE <= 6, preferring MACHINE/CABLE.
//	MILD:     exclude primary or secondary load on a high-impact exercise;
//	          other load is kept at RPE <= 7 with a caution.
//
// An unknown severity is treated as SEVERE.
func EvaluateInjuries(ex *ExerciseTraits, injuries []Injury) Verdict {
	var v Verdict
	for i := range injuries {
		inj := &injuries[i]
		muscles := inj.Muscles()
		primaryHit := muscles[ex.PrimaryMuscle]
		secondaryHit := false
		for _, m := range ex.SecondaryMuscles {
			if muscles[m] {
				secondaryHit = true
				break
			}
		}
		if !primaryHit && !secondaryHit {
			continue
		}

		label := inj.Label()
		switch inj.Severity {
		case taxonomy.InjurySeverityMild:
			if ex.IsHighImpact() {
				v.exclude(label + ": high-impact load on the injured area")
				continue
			}
			v.limit(mildMaxRPE, label+": caution, keep RPE <= 7")
		case taxonomy.InjurySeverityModerate:
			switch {
			case primaryHit:
				v.exclude(label + ": primary muscle is in the injured area")
			case ex.IsHighImpact():
				v.exclude(label + ": high-impact secondary load on the injured area")
			default:
				v.limit(moderateSecondaryMaxRPE, label+": secondary load, keep RPE <= 6, prefer machine or cable")
				v.PreferMachineOrCable = true
			}
		default:
			v.exclude(label + ": loads the injured area")
		}
	}
	return v
}

// DefaultGymEquipment is assumed when a profile lists no equipment (§8.3).
func DefaultGymEquipment() []taxonomy.Equipment {
	return []taxonomy.Equipment{
		taxonomy.EquipmentBodyOnly, taxonomy.EquipmentDumbbell, taxonomy.EquipmentBarbell,
		taxonomy.EquipmentCable, taxonomy.EquipmentMachine,
	}
}

// EquipmentAvailable reports whether the user owns the equipment; BODY_ONLY
// always counts as available and OTHER only when listed (§8.3).
func EquipmentAvailable(e taxonomy.Equipment, available []taxonomy.Equipment) bool {
	if e == taxonomy.EquipmentBodyOnly {
		return true
	}
	for _, a := range available {
		if a == e {
			return true
		}
	}
	return false
}

// UserLevel maps a profile experience level to the highest exercise level it
// may train; ADVANCED maps to EXPERT. An unknown level returns "" (no cap).
func UserLevel(experience string) taxonomy.Level {
	switch strings.ToUpper(strings.TrimSpace(experience)) {
	case "BEGINNER":
		return taxonomy.LevelBeginner
	case "INTERMEDIATE":
		return taxonomy.LevelIntermediate
	case "ADVANCED", "EXPERT":
		return taxonomy.LevelExpert
	default:
		return ""
	}
}

// LevelAllowed enforces exercise level <= user level (§8.4); an empty level on
// either side does not restrict.
func LevelAllowed(exercise, user taxonomy.Level) bool {
	ex, u := levelRank(exercise), levelRank(user)
	if ex == 0 || u == 0 {
		return true
	}
	return ex <= u
}

func levelRank(l taxonomy.Level) int {
	for i, known := range taxonomy.AllLevels() {
		if l == known {
			return i + 1
		}
	}
	return 0
}

// Athlete is what eligibility needs to know about the user.
type Athlete struct {
	Equipment []taxonomy.Equipment
	Level     taxonomy.Level
	Injuries  []Injury
}

// maxSeverityAtLeastModerate reports whether any injury is MODERATE or worse.
func (a *Athlete) maxSeverityAtLeastModerate() bool {
	for _, inj := range a.Injuries {
		if inj.Severity != taxonomy.InjurySeverityMild {
			return true
		}
	}
	return false
}

// ReplacementCandidate is a catalog exercise that may replace another.
type ReplacementCandidate struct {
	ID     string
	Traits ExerciseTraits
}

// RankReplacements returns the safe substitutes for original, best first
// (§8.6). A candidate must pass §8.2 and §8.3, have level <= the athlete's and
// not be stretching. Tiers: 1) same primary muscle and force, 2) same primary
// muscle, 3) primary muscle in sessionTargets and same force; candidates in no
// tier are dropped. Within a tier the same mechanic comes first, then, when an
// injury is MODERATE or worse, MACHINE/CABLE.
func RankReplacements(
	originalID string,
	original *ExerciseTraits,
	candidates []ReplacementCandidate,
	sessionTargets []taxonomy.MuscleGroup,
	athlete *Athlete,
) []ReplacementCandidate {
	targets := make(map[taxonomy.MuscleGroup]bool, len(sessionTargets))
	for _, m := range sessionTargets {
		targets[m] = true
	}
	preferGuided := athlete.maxSeverityAtLeastModerate()

	type ranked struct {
		candidate ReplacementCandidate
		key       [3]int
	}
	out := make([]ranked, 0, len(candidates))
	for i := range candidates {
		c := &candidates[i]
		t := &c.Traits
		if c.ID == originalID || t.IsWarmUpCoolDown() ||
			!LevelAllowed(t.Level, athlete.Level) || !EquipmentAvailable(t.Equipment, athlete.Equipment) {
			continue
		}
		if v := EvaluateInjuries(t, athlete.Injuries); v.Excluded {
			continue
		}

		samePrimary := t.PrimaryMuscle == original.PrimaryMuscle
		sameForce := t.Force == original.Force
		var tier int
		switch {
		case samePrimary && sameForce:
			tier = 1
		case samePrimary:
			tier = 2
		case targets[t.PrimaryMuscle] && sameForce:
			tier = 3
		default:
			continue
		}

		key := [3]int{tier, 0, 0}
		if t.Mechanic != original.Mechanic {
			key[1] = 1
		}
		if preferGuided && !t.IsMachineOrCable() {
			key[2] = 1
		}
		out = append(out, ranked{candidate: *c, key: key})
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].key, out[j].key
		for k := range a {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})

	result := make([]ReplacementCandidate, len(out))
	for i := range out {
		result[i] = out[i].candidate
	}
	return result
}
