package vo_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestFoodNutrient_CalculateGramsForCalories(t *testing.T) {
	t.Parallel()

	food := vo.NewFoodNutrient("f-1", "Ức gà", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET")

	// Target 330 kcal -> 200g
	got := food.CalculateGramsForCalories(330.0)
	want := 200.0
	if diff := got - want; diff > 0.001 || diff < -0.001 {
		t.Fatalf("got grams %f, want %f", got, want)
	}

	// Zero calories per 100g fallback
	zeroCalFood := vo.NewFoodNutrient("f-2", "Nước lọc", "BEVERAGE", 0.0, 0.0, 0.0, 0.0, nil, "", "", false, 0, "BUDGET")
	if gotZero := zeroCalFood.CalculateGramsForCalories(100.0); gotZero != 100.0 {
		t.Fatalf("got grams for zero cal food %f, want 100.0", gotZero)
	}
}

func TestFoodNutrient_MatchesSourceOrCategory(t *testing.T) {
	t.Parallel()

	food := vo.NewFoodNutrient("f-1", "Ức gà tươi", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET")

	if !food.MatchesSourceOrCategory("protein") {
		t.Fatalf("expected to match category protein")
	}
	if !food.MatchesSourceOrCategory("chicken") {
		t.Fatalf("expected to match protein source chicken")
	}
	if food.MatchesSourceOrCategory("sweet_potato") {
		t.Fatalf("did not expect to match sweet_potato")
	}
}

func TestValidateFoodItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		giveName     string
		giveCategory string
		givePrice    float64
		giveCostTier string
		wantTier     string
		wantErr      error
	}{
		{
			name:         "empty name returns error",
			giveName:     "   ",
			giveCategory: "PROTEIN",
			givePrice:    12000,
			giveCostTier: "BUDGET",
			wantErr:      vo.ErrEmptyFoodName,
		},
		{
			name:         "empty category returns error",
			giveName:     "Ức gà",
			giveCategory: "",
			givePrice:    12000,
			giveCostTier: "BUDGET",
			wantErr:      vo.ErrEmptyFoodCategory,
		},
		{
			name:         "negative price returns error",
			giveName:     "Ức gà",
			giveCategory: "PROTEIN",
			givePrice:    -100,
			giveCostTier: "BUDGET",
			wantErr:      vo.ErrNegativePrice,
		},
		{
			name:         "invalid tier returns error",
			giveName:     "Ức gà",
			giveCategory: "PROTEIN",
			givePrice:    10000,
			giveCostTier: "LUXURY",
			wantErr:      vo.ErrInvalidCostTier,
		},
		{
			name:         "valid budget tier",
			giveName:     "Ức gà tươi",
			giveCategory: "PROTEIN",
			givePrice:    12000,
			giveCostTier: "BUDGET",
			wantTier:     vo.CostTierBudget,
			wantErr:      nil,
		},
		{
			name:         "valid standard lowercase",
			giveName:     "Thịt bò",
			giveCategory: "PROTEIN",
			givePrice:    25000,
			giveCostTier: "standard",
			wantTier:     vo.CostTierStandard,
			wantErr:      nil,
		},
		{
			name:         "valid premium with whitespace",
			giveName:     "Cá hồi Nauy",
			giveCategory: "PROTEIN",
			givePrice:    50000,
			giveCostTier: "  PREMIUM  ",
			wantTier:     vo.CostTierPremium,
			wantErr:      nil,
		},
		{
			name:         "empty tier defaults to standard",
			giveName:     "Gạo lứt",
			giveCategory: "CARB",
			givePrice:    15000,
			giveCostTier: "",
			wantTier:     vo.CostTierStandard,
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tier, err := vo.ValidateFoodItem(tt.giveName, tt.giveCategory, tt.givePrice, tt.giveCostTier)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("got nil error, want %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}
			if tier != tt.wantTier {
				t.Errorf("got tier %q, want %q", tier, tt.wantTier)
			}
		})
	}
}

func TestNormalizeCostTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		giveTier string
		wantTier string
	}{
		{name: "standard uppercase", giveTier: "STANDARD", wantTier: vo.CostTierStandard},
		{name: "budget lowercase", giveTier: "budget", wantTier: vo.CostTierBudget},
		{name: "premium padded", giveTier: "  premium ", wantTier: vo.CostTierPremium},
		{name: "unknown fallback to standard", giveTier: "UNKNOWN", wantTier: vo.CostTierStandard},
		{name: "empty fallback to standard", giveTier: "", wantTier: vo.CostTierStandard},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := vo.NormalizeCostTier(tt.giveTier)
			if got != tt.wantTier {
				t.Errorf("got tier %q, want %q", got, tt.wantTier)
			}
		})
	}
}
