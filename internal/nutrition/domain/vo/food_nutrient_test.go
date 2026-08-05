package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestFoodNutrient_CalculateGramsForCalories(t *testing.T) {
	t.Parallel()

	food := vo.NewFoodNutrient("f-1", "Ức gà", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false)

	// Target 330 kcal -> 200g
	got := food.CalculateGramsForCalories(330.0)
	want := 200.0
	if diff := got - want; diff > 0.001 || diff < -0.001 {
		t.Fatalf("got grams %f, want %f", got, want)
	}

	// Zero calories per 100g fallback
	zeroCalFood := vo.NewFoodNutrient("f-2", "Nước lọc", "BEVERAGE", 0.0, 0.0, 0.0, 0.0, nil, "", "", false)
	if gotZero := zeroCalFood.CalculateGramsForCalories(100.0); gotZero != 100.0 {
		t.Fatalf("got grams for zero cal food %f, want 100.0", gotZero)
	}
}

func TestFoodNutrient_MatchesSourceOrCategory(t *testing.T) {
	t.Parallel()

	food := vo.NewFoodNutrient("f-1", "Ức gà tươi", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false)

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
