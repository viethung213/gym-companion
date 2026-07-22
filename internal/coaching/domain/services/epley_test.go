package services

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func TestCalculate1RM(t *testing.T) {
	t.Parallel()

	type giveStruct struct {
		weight float64
		reps   int
	}

	tests := []struct {
		name string
		give giveStruct
		want float64
	}{
		{
			name: "weight zero",
			give: giveStruct{weight: 0, reps: 10},
			want: 0,
		},
		{
			name: "negative weight",
			give: giveStruct{weight: -50, reps: 5},
			want: 0,
		},
		{
			name: "reps zero",
			give: giveStruct{weight: 100, reps: 0},
			want: 0,
		},
		{
			name: "negative reps",
			give: giveStruct{weight: 100, reps: -2},
			want: 0,
		},
		{
			name: "reps equal 1",
			give: giveStruct{weight: 100, reps: 1},
			want: 100,
		},
		{
			name: "reps equal 10 standard weight",
			give: giveStruct{weight: 100, reps: 10},
			want: 133.33333333333334,
		},
		{
			name: "reps equal 5",
			give: giveStruct{weight: 80, reps: 5},
			want: 93.33333333333333,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Calculate1RM(tt.give.weight, tt.give.reps)
			if !almostEqual(got, tt.want) {
				t.Errorf("Calculate1RM(%v, %v) got %v, want %v", tt.give.weight, tt.give.reps, got, tt.want)
			}
		})
	}
}

func TestCalculateSuggestedWeight(t *testing.T) {
	t.Parallel()

	type giveStruct struct {
		oneRM      float64
		targetReps int
	}

	tests := []struct {
		name string
		give giveStruct
		want float64
	}{
		{
			name: "oneRM zero",
			give: giveStruct{oneRM: 0, targetReps: 10},
			want: 0,
		},
		{
			name: "negative oneRM",
			give: giveStruct{oneRM: -100, targetReps: 5},
			want: 0,
		},
		{
			name: "targetReps zero",
			give: giveStruct{oneRM: 100, targetReps: 0},
			want: 0,
		},
		{
			name: "negative targetReps",
			give: giveStruct{oneRM: 100, targetReps: -1},
			want: 0,
		},
		{
			name: "targetReps equal 1",
			give: giveStruct{oneRM: 100, targetReps: 1},
			want: 100,
		},
		{
			name: "targetReps equal 10",
			give: giveStruct{oneRM: 133.33333333333334, targetReps: 10},
			want: 100,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CalculateSuggestedWeight(tt.give.oneRM, tt.give.targetReps)
			if !almostEqual(got, tt.want) {
				t.Errorf("CalculateSuggestedWeight(%v, %v) got %v, want %v", tt.give.oneRM, tt.give.targetReps, got, tt.want)
			}
		})
	}
}

func TestCalculateSuggestedReps(t *testing.T) {
	t.Parallel()

	type giveStruct struct {
		oneRM        float64
		targetWeight float64
	}

	tests := []struct {
		name string
		give giveStruct
		want int
	}{
		{
			name: "oneRM zero",
			give: giveStruct{oneRM: 0, targetWeight: 100},
			want: 0,
		},
		{
			name: "targetWeight zero",
			give: giveStruct{oneRM: 100, targetWeight: 0},
			want: 0,
		},
		{
			name: "negative input",
			give: giveStruct{oneRM: -100, targetWeight: 50},
			want: 0,
		},
		{
			name: "targetWeight equal to oneRM",
			give: giveStruct{oneRM: 100, targetWeight: 100},
			want: 1,
		},
		{
			name: "targetWeight greater than oneRM",
			give: giveStruct{oneRM: 100, targetWeight: 120},
			want: 1,
		},
		{
			name: "suggested reps for 10 reps weight",
			give: giveStruct{oneRM: 133.33333333333334, targetWeight: 100},
			want: 10,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CalculateSuggestedReps(tt.give.oneRM, tt.give.targetWeight)
			if got != tt.want {
				t.Errorf("CalculateSuggestedReps(%v, %v) got %v, want %v", tt.give.oneRM, tt.give.targetWeight, got, tt.want)
			}
		})
	}
}
