package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
)

func TestFoodItem_Lifecycle(t *testing.T) {
	t.Parallel()

	food, err := aggregate.NewFoodItem(
		"food-1", "Ức gà tươi", "PROTEIN",
		165.0, 31.0, 0.0, 3.6,
		[]string{"PorkFree"}, "CHICKEN", "", false,
		13000, "BUDGET",
	)
	if err != nil {
		t.Fatalf("unexpected error creating food item: %v", err)
	}

	if got := food.Status(); got != aggregate.FoodStatusDraft {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusDraft)
	}

	if err := food.SubmitForApproval(); err != nil {
		t.Fatalf("unexpected error submitting for approval: %v", err)
	}
	if got := food.Status(); got != aggregate.FoodStatusPendingApproval {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusPendingApproval)
	}

	if err := food.Approve(); err != nil {
		t.Fatalf("unexpected error approving: %v", err)
	}
	if got := food.Status(); got != aggregate.FoodStatusActive {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusActive)
	}
}

func TestFoodItem_Reject(t *testing.T) {
	t.Parallel()

	food, err := aggregate.NewFoodItem("food-2", "Thịt heo", "PROTEIN", 240, 26, 0, 14, nil, "PORK", "", false, 16000, "STANDARD")
	if err != nil {
		t.Fatalf("unexpected error creating food item: %v", err)
	}
	_ = food.SubmitForApproval()

	if err := food.Reject(); err != nil {
		t.Fatalf("unexpected error rejecting: %v", err)
	}
	if got := food.Status(); got != aggregate.FoodStatusDraft {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusDraft)
	}
}

func TestFoodItem_InvalidTransitions(t *testing.T) {
	t.Parallel()

	food, err := aggregate.NewFoodItem("food-3", "Rau bina", "VEGGIE", 23, 2.9, 3.6, 0.4, nil, "", "", false, 4000, "BUDGET")
	if err != nil {
		t.Fatalf("unexpected error creating food item: %v", err)
	}

	if err := food.Approve(); err == nil {
		t.Fatalf("expected error approving draft item, got nil")
	}

	if err := food.Reject(); err == nil {
		t.Fatalf("expected error rejecting draft item, got nil")
	}

	_ = food.Archive()
	if got := food.Status(); got != aggregate.FoodStatusArchived {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusArchived)
	}
}

func TestFoodItem_Validation(t *testing.T) {
	t.Parallel()

	// 1. Empty name
	_, err := aggregate.NewFoodItem("f-empty-name", "", "PROTEIN", 100, 20, 0, 2, nil, "", "", false, 10000, "STANDARD")
	if err == nil {
		t.Fatalf("expected error on empty name, got nil")
	}

	// 2. Empty category
	_, err = aggregate.NewFoodItem("f-empty-cat", "Gà", "   ", 100, 20, 0, 2, nil, "", "", false, 10000, "STANDARD")
	if err == nil {
		t.Fatalf("expected error on empty category, got nil")
	}

	// 3. Negative price
	_, err = aggregate.NewFoodItem("f-neg", "Gà", "PROTEIN", 100, 20, 0, 2, nil, "", "", false, -100, "STANDARD")
	if err == nil {
		t.Fatalf("expected error on negative price, got nil")
	}

	// 4. Invalid cost tier
	_, err = aggregate.NewFoodItem("f-tier", "Gà", "PROTEIN", 100, 20, 0, 2, nil, "", "", false, 10000, "UNKNOWN_TIER")
	if err == nil {
		t.Fatalf("expected error on invalid cost tier, got nil")
	}

	// 5. Valid food item
	food, err := aggregate.NewFoodItem("f-ok", "Gà", "PROTEIN", 100, 20, 0, 2, nil, "", "", false, 12000, "budget")
	if err != nil {
		t.Fatalf("unexpected error on valid food item: %v", err)
	}
	if food.CostTier() != "BUDGET" {
		t.Fatalf("got CostTier %q, want %q", food.CostTier(), "BUDGET")
	}

	// 6. SetPricing
	if err := food.SetPricing(15000, "PREMIUM"); err != nil {
		t.Fatalf("unexpected error setting pricing: %v", err)
	}
	if food.CostTier() != "PREMIUM" || food.PricePer100g() != 15000 {
		t.Fatalf("got CostTier %q, Price %v; want PREMIUM, 15000", food.CostTier(), food.PricePer100g())
	}
}

func TestFoodItem_Reconstruct(t *testing.T) {
	t.Parallel()

	now := time.Now()
	food := aggregate.ReconstructFoodItem(
		"food-4", "Khoai lang", "CARB", 90, 2, 21, 0.1, []string{"GlutenFree"},
		"", "SWEET_POTATO", false, 4500.0, "BUDGET", aggregate.FoodStatusActive, now, now,
	)

	if got := food.ID(); got != "food-4" {
		t.Fatalf("got ID %q, want %q", got, "food-4")
	}
	if got := food.Name(); got != "Khoai lang" {
		t.Fatalf("got Name %q, want %q", got, "Khoai lang")
	}
	if got := len(food.AllergenTags()); got != 1 {
		t.Fatalf("got allergen tags len %d, want 1", got)
	}
}
