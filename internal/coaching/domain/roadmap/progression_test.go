package roadmap

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

func TestRepRangeForGoal(t *testing.T) {
	for goal, want := range map[string]RepRange{
		"BUILD_MUSCLE": {8, 12}, "hypertrophy": {8, 12}, "STRENGTH": {4, 6},
		"ENDURANCE": {12, 15}, "LOSE_FAT": {10, 15}, "": {8, 12}, "unknown": {8, 12},
		// Reproduces the review finding: the goals stored by the app are
		// FAT_LOSS and PRIMARY_GOAL_*, which matched nothing and fell back to
		// hypertrophy.
		"FAT_LOSS": {10, 15}, "weight_loss": {10, 15}, "PRIMARY_GOAL_STRENGTH": {4, 6},
		"PRIMARY_GOAL_MUSCLE_GAIN": {8, 12}, "endurance": {12, 15},
	} {
		if got := RepRangeForGoal(goal); got != want {
			t.Errorf("%q: %v, want %v", goal, got, want)
		}
	}
}

func TestLoadStep(t *testing.T) {
	for eq, want := range map[taxonomy.Equipment]float32{
		taxonomy.EquipmentDumbbell: 2, taxonomy.EquipmentKettlebells: 2,
		taxonomy.EquipmentBarbell: 2.5, taxonomy.EquipmentEZCurlBar: 2.5,
		taxonomy.EquipmentMachine: 2.5, taxonomy.EquipmentCable: 2.5,
		taxonomy.EquipmentBands: 0, taxonomy.EquipmentBodyOnly: 0,
	} {
		if got := LoadStep(eq); got != want {
			t.Errorf("%s: %v, want %v", eq, got, want)
		}
	}
}

func TestProgressLine(t *testing.T) {
	hypertrophy := RepRange{8, 12}
	line := func(sets, reps int32, kg float32, sec int32) PrescribedExercise {
		return PrescribedExercise{ExerciseID: "ex", TargetSets: sets, TargetReps: reps, TargetWeight: kg, DurationSeconds: sec, TargetRPE: 7}
	}
	tests := []struct {
		name string
		in   PrescribedExercise
		step float32
		rng  RepRange
		ceil float32
		want PrescribedExercise
	}{
		{"small dumbbell: reps first", line(3, 8, 15, 0), 2, hypertrophy, 0, line(3, 10, 15, 0)},
		{"top of the range: one step up, reps back to the floor", line(3, 12, 15, 0), 2, hypertrophy, 0, line(3, 8, 17, 0)},
		{"near the top: reps capped at the top", line(3, 11, 15, 0), 2, hypertrophy, 0, line(3, 12, 15, 0)},
		{"barbell takes one plate step", line(4, 12, 50, 0), 2.5, hypertrophy, 0, line(4, 8, 52.5, 0)},
		{"strength range climbs one rep", line(5, 4, 100, 0), 2.5, RepRange{4, 6}, 0, line(5, 5, 100, 0)},
		{"over the record ceiling: unchanged", line(3, 12, 50, 0), 2.5, hypertrophy, 51, line(3, 12, 50, 0)},
		{"no load step (bands): reps only, then held", line(3, 12, 5, 0), 0, hypertrophy, 0, line(3, 12, 5, 0)},
		{"bodyweight: +2 reps", line(3, 10, 0, 0), 0, hypertrophy, 0, line(3, 12, 0, 0)},
		{"bodyweight capped at 20", line(3, 19, 0, 0), 0, hypertrophy, 0, line(3, 20, 0, 0)},
		{"hold: +5 s", line(3, 0, 0, 40), 0, hypertrophy, 0, line(3, 0, 0, 45)},
		{"hold capped at 90 s", line(3, 0, 0, 88), 0, hypertrophy, 0, line(3, 0, 0, 90)},
		// Reproduces the review finding: the 300 kg hard limit (BR-AC-13) was
		// not checked, so the guardrail refused the whole adaptation.
		{"never past the 300 kg limit", line(3, 12, 299, 0), 2.5, hypertrophy, 0, line(3, 12, 299, 0)},
		// Reproduces the review finding: a line prescribed outside the goal's
		// range was pulled into it, so a heavy 5-rep line climbed to 12 reps
		// at the same load, and a 15-rep line dropped to 8. It keeps the reps
		// it was prescribed and takes the load step.
		{"heavy line below the range keeps its reps", line(5, 5, 100, 0), 2.5, hypertrophy, 0, line(5, 5, 102.5, 0)},
		{"light line above the range keeps its reps", line(3, 15, 20, 0), 2, hypertrophy, 0, line(3, 15, 22, 0)},
		{"below the range, no load step: held", line(3, 5, 5, 0), 0, hypertrophy, 0, line(3, 5, 5, 0)},
	}
	for _, tt := range tests {
		if got := ProgressLine(&tt.in, tt.step, tt.rng, tt.ceil); got != tt.want {
			t.Errorf("%s:\n got %+v\nwant %+v", tt.name, got, tt.want)
		}
	}
}

// The upcoming sessions progress from what they prescribed before the model
// rewrote them; a DELOAD week, an exercise without a rule (an injury limits it)
// and an exercise the model swapped in keep the model's numbers.
func TestRoadmap_ProgressUpcoming(t *testing.T) {
	rm := deloadFixture(t, 0, [][]int32{{2}}, []SessionPlanStatus{SessionPlanStatusPending})
	peak, _ := rm.FindSession("peak")
	peak.info.Prescription.MainExercises = []PrescribedExercise{
		{ExerciseID: "db-press", TargetSets: 3, TargetReps: 12, TargetWeight: 14, TargetRPE: 8.5},
		{ExerciseID: "limited", TargetSets: 3, TargetReps: 8, TargetWeight: 40, TargetRPE: 6},
		{ExerciseID: "swapped-in", TargetSets: 3, TargetReps: 8, TargetWeight: 20, TargetRPE: 8.5},
	}
	before := map[string][]PrescribedExercise{
		"peak": {
			{ExerciseID: "db-press", TargetSets: 3, TargetReps: 12, TargetWeight: 14},
			{ExerciseID: "limited", TargetSets: 3, TargetReps: 8, TargetWeight: 40},
		},
		"dl1": {{ExerciseID: "dl1-exa", TargetSets: 2, TargetReps: 8}},
	}
	rules := map[string]LineProgression{"db-press": {LoadStep: 2}, "dl1-exa": {LoadStep: 2}, "swapped-in": {LoadStep: 2}}

	changed, err := rm.ProgressUpcoming(before, RepRange{8, 12}, rules, beforeTheBlock())
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "peak" {
		t.Errorf("changed %v, want only the PEAK session", changed)
	}
	mains := peak.Info().Prescription.MainExercises
	if m := mains[0]; m.TargetWeight != 16 || m.TargetReps != 8 || m.TargetRPE != 8.5 {
		t.Errorf("db-press %+v, want 16 kg x 8 at the model's RPE 8.5", m)
	}
	if m := mains[1]; m.TargetWeight != 40 || m.TargetReps != 8 {
		t.Errorf("injury-limited line %+v, want it unchanged", m)
	}
	if m := mains[2]; m.TargetWeight != 20 || m.TargetReps != 8 {
		t.Errorf("swapped-in line %+v, want the model's numbers", m)
	}
	if got := setsOf(t, rm, "dl1"); !equalSets(got, []int32{2}) {
		t.Errorf("DELOAD sets %v, want untouched", got)
	}
}
