package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
)

func TestFoodItem_Lifecycle(t *testing.T) {
	t.Parallel()

	food := aggregate.NewFoodItem(
		"food-1", "Ức gà tươi", "PROTEIN",
		165.0, 31.0, 0.0, 3.6,
		[]string{"PorkFree"}, "CHICKEN", "", false,
	)

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

	food := aggregate.NewFoodItem("food-2", "Thịt heo", "PROTEIN", 240, 26, 0, 14, nil, "PORK", "", false)
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

	food := aggregate.NewFoodItem("food-3", "Rau bina", "VEGGIE", 23, 2.9, 3.6, 0.4, nil, "", "", false)

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

func TestFoodItem_Reconstruct(t *testing.T) {
	t.Parallel()

	now := time.Now()
	food := aggregate.ReconstructFoodItem(
		"food-4", "Khoai lang", "CARB", 90, 2, 21, 0.1, []string{"GlutenFree"},
		"", "SWEET_POTATO", false, aggregate.FoodStatusActive, now, now,
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
