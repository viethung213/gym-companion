package vo_test

import (
	"math"
	"testing"

	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

func TestCalculateLevel(t *testing.T) {
	tests := []struct {
		name string
		give int64
		want int32
	}{
		{name: "negative xp returns level 1", give: -100, want: 1},
		{name: "0 xp returns level 1", give: 0, want: 1},
		{name: "49 xp returns level 1", give: 49, want: 1},
		{name: "50 xp returns level 2", give: 50, want: 2},
		{name: "199 xp returns level 2", give: 199, want: 2},
		{name: "200 xp returns level 3", give: 200, want: 3},
		{name: "450 xp returns level 4", give: 450, want: 4},
		{name: "Level 10 threshold (50 * 9^2 = 4050)", give: 4050, want: 10},
		{name: "Level 100 threshold (50 * 99^2 = 490050)", give: 490050, want: 100},
		{name: "Huge xp is capped at level 100", give: 10000000, want: 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.CalculateLevel(tt.give)
			if got != tt.want {
				t.Errorf("CalculateLevel(%d) = %d, want %d", tt.give, got, tt.want)
			}
		})
	}
}

func TestXpRequiredForLevel(t *testing.T) {
	tests := []struct {
		name string
		give int32
		want int64
	}{
		{name: "level 0 gives 0 xp", give: 0, want: 0},
		{name: "level 1 gives 0 xp", give: 1, want: 0},
		{name: "level 2 gives 50 xp", give: 2, want: 50},
		{name: "level 3 gives 200 xp", give: 3, want: 200},
		{name: "level 10 gives 4050 xp", give: 10, want: 4050},
		{name: "level 100 gives 490050 xp", give: 100, want: 490050},
		{name: "level > 100 capped at level 100 threshold", give: 150, want: 490050},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.XpRequiredForLevel(tt.give)
			if got != tt.want {
				t.Errorf("XpRequiredForLevel(%d) = %d, want %d", tt.give, got, tt.want)
			}
		})
	}
}

func TestXpToNextLevel(t *testing.T) {
	tests := []struct {
		name string
		give int64
		want int64
	}{
		{name: "0 xp needs 50 to reach level 2", give: 0, want: 50},
		{name: "30 xp needs 20 to reach level 2", give: 30, want: 20},
		{name: "50 xp needs 150 to reach level 3 (200 - 50)", give: 50, want: 150},
		{name: "At or above max level returns 0", give: 500000, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.XpToNextLevel(tt.give)
			if got != tt.want {
				t.Errorf("XpToNextLevel(%d) = %d, want %d", tt.give, got, tt.want)
			}
		})
	}
}

func TestProgressPercentage(t *testing.T) {
	tests := []struct {
		name string
		give int64
		want float64
	}{
		{name: "0 xp in level 1 bracket (0..50) is 0%", give: 0, want: 0.0},
		{name: "25 xp in level 1 bracket (0..50) is 50%", give: 25, want: 50.0},
		{name: "50 xp is start of level 2 bracket (50..200) -> 0%", give: 50, want: 0.0},
		{name: "125 xp is midpoint of level 2 bracket (50..200) -> 50%", give: 125, want: 50.0},
		{name: "At max level is 100%", give: 500000, want: 100.0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.ProgressPercentage(tt.give)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ProgressPercentage(%d) = %f, want %f", tt.give, got, tt.want)
			}
		})
	}
}
