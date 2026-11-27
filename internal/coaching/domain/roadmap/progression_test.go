package roadmap

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// The band follows the reps the coach prescribed; climbing inside it never
// leaves it, so a heavy line stays heavy and a light one keeps its rep buffer.
func TestRepRangeFor(t *testing.T) {
	for reps, want := range map[int32]RepRange{
		1: {4, 6}, 5: {4, 6}, 6: {4, 6}, 7: {8, 12}, 10: {8, 12}, 12: {8, 12},
		13: {15, 20}, 15: {15, 20}, 25: {15, 20},
	} {
		if got := RepRangeFor(reps); got != want {
			t.Errorf("%d reps: %v, want %v", reps, got, want)
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
	line := func(sets, reps int32, kg float32, sec int32) PrescribedExercise {
		return PrescribedExercise{ExerciseID: "ex", TargetSets: sets, TargetReps: reps, TargetWeight: kg, DurationSeconds: sec, TargetRPE: 7}
	}
	tests := []struct {
		name string
		in   PrescribedExercise
		step float32
		ceil float32
		want PrescribedExercise
	}{
		{"8-12: reps first", line(3, 8, 15, 0), 2, 0, line(3, 10, 15, 0)},
		{"8-12 top: one step up, reps back to 8", line(3, 12, 15, 0), 2, 0, line(3, 8, 17, 0)},
		{"near the top: reps capped at the top", line(3, 11, 15, 0), 2, 0, line(3, 12, 15, 0)},
		{"barbell takes one plate step", line(4, 12, 50, 0), 2.5, 0, line(4, 8, 52.5, 0)},
		{"7 reps climbs into its 8-12 band", line(3, 7, 30, 0), 2.5, 0, line(3, 9, 30, 0)},
		// Reproduces the review finding: a heavy 5-rep line climbed toward 12
		// reps at the same load. It stays in 4-6.
		{"heavy 5s climb one rep", line(5, 5, 100, 0), 2.5, 0, line(5, 6, 100, 0)},
		{"heavy 6s: one step up, back to 4", line(5, 6, 100, 0), 2.5, 0, line(5, 4, 102.5, 0)},
		// A light high-rep line earns its load step at 20 reps instead of
		// jumping 6 -> 8 kg (+33%) at 15.
		{"light 15s climb reps", line(3, 15, 6, 0), 2, 0, line(3, 17, 6, 0)},
		{"light 20s: one step up, back to 15", line(3, 20, 6, 0), 2, 0, line(3, 15, 8, 0)},
		{"13 reps climbs into 15-20", line(3, 13, 6, 0), 2, 0, line(3, 15, 6, 0)},
		{"over the record ceiling: unchanged", line(3, 12, 50, 0), 2.5, 51, line(3, 12, 50, 0)},
		{"no load step (bands): held at the top", line(3, 12, 5, 0), 0, 0, line(3, 12, 5, 0)},
		{"loaded without reps: unchanged", line(3, 0, 20, 0), 2, 0, line(3, 0, 20, 0)},
		{"bodyweight: +2 reps", line(3, 10, 0, 0), 0, 0, line(3, 12, 0, 0)},
		{"bodyweight capped at 20", line(3, 19, 0, 0), 0, 0, line(3, 20, 0, 0)},
		{"hold: +5 s", line(3, 0, 0, 40), 0, 0, line(3, 0, 0, 45)},
		{"hold capped at 90 s", line(3, 0, 0, 88), 0, 0, line(3, 0, 0, 90)},
		// The 300 kg hard limit (BR-AC-13) holds: the guardrail would refuse
		// the whole adaptation.
		{"never past the 300 kg limit", line(3, 12, 299, 0), 2.5, 0, line(3, 12, 299, 0)},
	}
	for _, tt := range tests {
		if got := ProgressLine(&tt.in, tt.step, tt.ceil); got != tt.want {
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

	changed, err := rm.ProgressUpcoming(before, rules, beforeTheBlock())
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
