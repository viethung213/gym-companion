package service

import (
	"math"
	"testing"
)

func TestSCRCalculator_SCR(t *testing.T) {

	c := NewSCRCalculator()

	tests := []struct {
		name string

		actual int

		prescribed int

		want float64
	}{

		{"100% completion", 16, 16, 100.0},

		{"50% completion", 8, 16, 50.0},

		{"80% completion", 8, 10, 80.0},

		{"zero prescribed → 0", 5, 0, 0},

		{"negative actual → 0%", -3, 10, 0},

		{"over-completion caps at 100", 20, 16, 100.0},
	}

	for _, tt := range tests {

		tt := tt

		t.Run(tt.name, func(t *testing.T) {

			t.Parallel()

			if got := c.SCR(tt.actual, tt.prescribed); math.Abs(got-tt.want) > 1e-9 {

				t.Errorf("SCR(%d, %d) = %v, want %v", tt.actual, tt.prescribed, got, tt.want)

			}

		})

	}

}

func TestSCRCalculator_DeltaRPE(t *testing.T) {

	c := NewSCRCalculator()

	if got := c.DeltaRPE(8.5, 7.0); math.Abs(got-1.5) > 1e-9 {

		t.Errorf("DeltaRPE(8.5, 7.0) = %v, want 1.5", got)

	}

	if got := c.DeltaRPE(6.0, 7.0); math.Abs(got-(-1.0)) > 1e-9 {

		t.Errorf("DeltaRPE(6.0, 7.0) = %v, want -1.0", got)

	}

}

func TestSCRCalculator_EpleyOneRepMax(t *testing.T) {

	c := NewSCRCalculator()

	tests := []struct {
		name string

		weight float64

		reps int

		want float64
	}{

		{"1 rep = weight", 100, 1, 100},

		{"5 reps at 100 → ~116.67", 100, 5, 100 * (1 + 5.0/30.0)},

		{"zero weight → 0", 0, 5, 0},

		{"zero reps → 0", 100, 0, 0},
	}

	for _, tt := range tests {

		tt := tt

		t.Run(tt.name, func(t *testing.T) {

			t.Parallel()

			if got := c.EpleyOneRepMax(tt.weight, tt.reps); math.Abs(got-tt.want) > 1e-6 {

				t.Errorf("Epley(%v, %d) = %v, want %v", tt.weight, tt.reps, got, tt.want)

			}

		})

	}

}

func TestSCRCalculator_WeeklySCR(t *testing.T) {

	c := NewSCRCalculator()

	actual := []int{16, 12, 14}

	prescribed := []int{16, 16, 16}

	got := c.WeeklySCR(actual, prescribed)

	want := 42.0 / 48.0 * 100.0

	if math.Abs(got-want) > 1e-6 {

		t.Errorf("WeeklySCR = %v, want %v", got, want)

	}

}

func TestRoundToNearest(t *testing.T) {

	tests := []struct {
		x, step, want float64
	}{

		{102.3, 2.5, 102.5},

		{101.4, 2.5, 102.5}, // 101.4/2.5 = 40.56 → round 41 → 102.5

		{101.24, 2.5, 100.0}, // 101.24/2.5 = 40.496 → round 40 → 100.0

		{75.0, 5.0, 75.0},

		{50.0, 0, 50.0},
	}

	for _, tt := range tests {

		if got := RoundToNearest(tt.x, tt.step); math.Abs(got-tt.want) > 1e-9 {

			t.Errorf("RoundToNearest(%v, %v) = %v, want %v", tt.x, tt.step, got, tt.want)

		}

	}

}
