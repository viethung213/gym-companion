package service

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestSelectPantryCandidates(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()
	planDate := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-pantry-1"

	// Case 1: Người dùng chỉ có Protein -> cần bổ sung Carb và Veggie
	userItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-1", "Ức gà hấp", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET"),
	}

	mandatory, supplementary := SelectPantryCandidates(userItems, catalog, nil, userID, planDate)
	if len(mandatory) != 1 || mandatory[0].Name() != "Ức gà hấp" {
		t.Fatalf("expected mandatory to be user's original ingredient, got %v", mandatory)
	}

	var hasCarb, hasVeggie bool
	for _, item := range supplementary {
		switch item.Category() {
		case "CARB":
			hasCarb = true
		case "VEGGIE":
			hasVeggie = true
		}
	}
	if !hasCarb || !hasVeggie {
		t.Errorf("expected supplementary to include both CARB and VEGGIE")
	}

	// Case 2: Đã có đủ cả 3 nhóm Protein + Carb + Veggie -> không bổ sung
	fullItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false, 13000, "BUDGET"),
		vo.NewFoodNutrient("p-2", "Khoai lang luộc", "CARB", 90, 2, 21, 0.1, nil, "", "", false, 4500, "BUDGET"),
		vo.NewFoodNutrient("p-3", "Bông cải xanh", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false, 9000, "STANDARD"),
	}
	mandFull, suppFull := SelectPantryCandidates(fullItems, catalog, nil, userID, planDate)
	if len(mandFull) != 3 || len(suppFull) != 0 {
		t.Errorf("expected 3 mandatory and 0 supplementary, got %d and %d", len(mandFull), len(suppFull))
	}

	// Case 3: Nguyên liệu chưa biết (UNKNOWN) -> giữ nguyên UNKNOWN, cung cấp pool DB an toàn để AI tự chọn nhóm thiếu
	unknownItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-unknown", "Thịt nai", "UNKNOWN", 0, 0, 0, 0, nil, "", "", false, 0, "BUDGET"),
	}
	mandUnknown, suppUnknown := SelectPantryCandidates(unknownItems, catalog, nil, userID, planDate)
	if len(mandUnknown) != 1 || mandUnknown[0].Category() != "UNKNOWN" {
		t.Errorf("expected category to remain UNKNOWN without backend hardcoded assumption, got %v", mandUnknown)
	}
	var hasSuppProtein, hasSuppCarb, hasSuppVeggie bool
	for _, item := range suppUnknown {
		switch item.Category() {
		case "PROTEIN":
			hasSuppProtein = true
		case "CARB":
			hasSuppCarb = true
		case "VEGGIE":
			hasSuppVeggie = true
		}
	}
	if !hasSuppProtein || !hasSuppCarb || !hasSuppVeggie {
		t.Errorf("expected supplementary to contain candidates across groups for AI to pick missing ones")
	}
}
