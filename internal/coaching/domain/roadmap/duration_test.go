package roadmap

import "testing"

func TestWorkoutPrescription_EstimatedMinutes(t *testing.T) {
	main := func(sets int32) PrescribedExercise {
		return PrescribedExercise{TargetSets: sets, TargetReps: 10, RestSetSec: 90, RestExerciseSec: 120}
	}
	warmUp := PrescribedExercise{TargetSets: 1, DurationSeconds: 300, RestExerciseSec: 60}

	tests := []struct {
		name string
		p    WorkoutPrescription
		want int32
	}{
		// 3 x (3 x 40s + 2 x 90s + 120s) = 1260s, + 360s warm-up = 27 min -> 30
		{"regular", WorkoutPrescription{WarmUps: []PrescribedExercise{warmUp},
			MainExercises: []PrescribedExercise{main(3), main(3), main(3)}}, 30},
		// 3 x (2 x 40s + 90s + 120s) = 870s, + 360s = 20.5 min -> 25
		{"express: fewer sets, shorter", WorkoutPrescription{WarmUps: []PrescribedExercise{warmUp},
			MainExercises: []PrescribedExercise{main(2), main(2), main(2)}}, 25},
		{"empty floors at the minimum", WorkoutPrescription{}, MinSessionMinutes},
		{"huge caps at the maximum", WorkoutPrescription{MainExercises: []PrescribedExercise{
			{TargetSets: 10, DurationSeconds: 1800, RestSetSec: 600}}}, MaxSessionMinutes},
		// neither reps nor duration: 30s per set
		{"unknown set length", WorkoutPrescription{MainExercises: []PrescribedExercise{
			{TargetSets: 4, RestSetSec: 60, RestExerciseSec: 60}}}, 10},
	}
	for _, tt := range tests {
		if got := tt.p.EstimatedMinutes(); got != tt.want {
			t.Errorf("%s: %d min, want %d", tt.name, got, tt.want)
		}
	}
}
