package adk

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

func testResolver() exerciseMetaResolver {
	muscles := map[string]string{
		"m-pec":   "Pectorals",
		"m-lat":   "Lats",
		"m-quad":  "Quads",
		"m-chest": "Chest",
	}
	bodyParts := map[string]string{
		"bp-chest": "Chest",
		"bp-back":  "Back",
		"bp-legs":  "Upper Legs",
	}
	equips := map[string]string{
		"eq-bb": "Barbell",
		"eq-bw": "Body Weight",
		"eq-db": "Dumbbell",
	}
	tags := map[string]string{
		"t-warm": tagMobilityWarmUp,
		"t-cool": tagStretchingCoolDown,
	}
	return exerciseMetaResolver{
		muscle: func(s string) string {
			if v, ok := muscles[s]; ok {
				return v
			}
			return s
		},
		bodyPart: func(s string) string {
			if v, ok := bodyParts[s]; ok {
				return v
			}
			return s
		},
		equipment: func(s string) string {
			if v, ok := equips[s]; ok {
				return v
			}
			return s
		},
		tag: func(s string) string {
			if v, ok := tags[s]; ok {
				return v
			}
			return s
		},
	}
}

func TestFilterEligibleExercises(t *testing.T) {
	r := testResolver()
	all := []port.Exercise{
		{ExerciseID: "ok-pec", Name: "Bench Press", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bb", Difficulty: "Intermediate"},
		{ExerciseID: "adv-pec", Name: "Planche Pushup", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bw", Difficulty: "Advanced"},
		{ExerciseID: "no-eq", Name: "DB Fly", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-db", Difficulty: "Beginner"},
		{ExerciseID: "inj-quad", Name: "Back Squat", MuscleGroup: "m-quad", BodyPart: "bp-legs", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "ok-lat-bw", Name: "Inverted Row", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bw", Difficulty: "Beginner", IsBodyweight: true},
	}

	profile := port.Profile{
		ExperienceLevel:    "INTERMEDIATE",
		AvailableEquipment: []string{"eq-bb"},
	}

	got := filterEligibleExercises(all, &profile, []string{"Upper Legs"}, r)
	if len(got) != 2 {
		t.Fatalf("got %d eligible exercises, want 2: %+v", len(got), got)
	}
	if got[0].ExerciseID != "ok-pec" || got[1].ExerciseID != "ok-lat-bw" {
		t.Errorf("got IDs [%s, %s], want [ok-pec, ok-lat-bw]", got[0].ExerciseID, got[1].ExerciseID)
	}
}

func TestSelectMainCandidates_DynamicDBAndPreferredEmphasis(t *testing.T) {
	r := testResolver()
	eligible := []port.Exercise{
		{ExerciseID: "p1", Name: "Bench 1", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "p2", Name: "Bench 2", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "p3", Name: "Bench 3", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "p4", Name: "Bench 4", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "l1", Name: "Row 1", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "l2", Name: "Row 2", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "l3", Name: "Row 3", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "l4", Name: "Row 4", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bb", Difficulty: "Beginner"},
		{ExerciseID: "c1", Name: "Sprint", MuscleGroup: "Cardiovascular System", BodyPart: "Cardio", Equipment: "eq-bw", Difficulty: "Beginner"},
		{ExerciseID: "stretch-pec", Name: "Chest Stretch", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-cool"}},
		{ExerciseID: "warm-pec", Name: "Dynamic Chest", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-warm"}},
	}

	profile := port.Profile{
		PrimaryGoal:           "BUILD_MUSCLE",
		PreferredMuscleGroups: []string{"m-chest"}, // resolves to "Chest" -> matches bp-chest
	}

	candidates, activeBP := selectMainCandidates(eligible, &profile, r)

	// Preferred Pectorals gets all 4 (under preferredMuscleQuota=8), excluding stretch & warmup tags.
	if len(candidates["Pectorals"]) != 4 {
		t.Errorf("got %d Pectorals exercises, want 4 (preferred quota, excluding recovery tags)", len(candidates["Pectorals"]))
	}
	// Non-preferred muscle Lats capped at defaultMuscleQuota (3).
	if len(candidates["Lats"]) != defaultMuscleQuota {
		t.Errorf("got %d Lats exercises, want %d (defaultMuscleQuota)", len(candidates["Lats"]), defaultMuscleQuota)
	}
	// Cardio skipped for BUILD_MUSCLE when not preferred.
	if len(candidates["Cardiovascular System"]) != 0 {
		t.Errorf("expected Cardio skipped for BUILD_MUSCLE, got %d", len(candidates["Cardiovascular System"]))
	}
	if !activeBP["Chest"] || !activeBP["Back"] {
		t.Errorf("expected activeBodyParts to contain Chest and Back, got %v", activeBP)
	}

	// FAT_LOSS automatically includes Cardio.
	fatLossProfile := port.Profile{PrimaryGoal: "FAT_LOSS"}
	fatLossCandidates, _ := selectMainCandidates(eligible, &fatLossProfile, r)
	if len(fatLossCandidates["Cardiovascular System"]) != 1 {
		t.Errorf("expected Cardio included for FAT_LOSS, got %d", len(fatLossCandidates["Cardiovascular System"]))
	}
}

func TestSelectRecoveryCandidates(t *testing.T) {
	r := testResolver()
	eligible := []port.Exercise{
		{ExerciseID: "w-gen", Name: "Ankle Circles", MuscleGroup: "Calves", BodyPart: "Lower Legs", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-warm"}},
		{ExerciseID: "w-chest", Name: "Dynamic Chest Stretch", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-warm"}},
		{ExerciseID: "act-back", Name: "Scapular Pull", MuscleGroup: "m-lat", BodyPart: "bp-back", Equipment: "eq-bw", Difficulty: "Beginner", IsBodyweight: true},
		{ExerciseID: "c-chest", Name: "Chest Stretch", MuscleGroup: "m-pec", BodyPart: "bp-chest", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-cool"}},
		{ExerciseID: "c-legs", Name: "Hamstring Stretch", MuscleGroup: "m-quad", BodyPart: "bp-legs", Equipment: "eq-bw", Difficulty: "Beginner", TagIDs: []string{"t-cool"}},
	}

	activeBP := map[string]bool{"Chest": true, "Back": true}
	warmUps, coolDowns := selectRecoveryCandidates(eligible, activeBP, r)

	if len(warmUps["Chest"]) != 1 || warmUps["Chest"][0][0] != "w-chest" {
		t.Errorf("expected w-chest in warmUps[Chest], got %v", warmUps["Chest"])
	}
	if len(warmUps["General"]) != 1 || warmUps["General"][0][0] != "w-gen" {
		t.Errorf("expected w-gen in warmUps[General], got %v", warmUps["General"])
	}
	if len(warmUps["Back"]) != 0 {
		t.Errorf("untagged main bodyweight exercise must not leak into warmUps[Back], got %v", warmUps["Back"])
	}
	if len(coolDowns["Chest"]) != 1 || coolDowns["Chest"][0][0] != "c-chest" {
		t.Errorf("expected c-chest in coolDowns[Chest], got %v", coolDowns["Chest"])
	}
	if len(coolDowns["General"]) != 1 || coolDowns["General"][0][0] != "c-legs" {
		t.Errorf("expected remaining stretch c-legs in coolDowns[General], got %v", coolDowns["General"])
	}
}
