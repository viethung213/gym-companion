package service

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func makeTestCatalog() []vo.FoodNutrient {
	return []vo.FoodNutrient{
		// Proteins
		vo.NewFoodNutrient("p1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000.0, "BUDGET"),
		vo.NewFoodNutrient("p2", "Thịt bò thăn", "PROTEIN", 250, 26, 0, 15, nil, "BEEF", "", false, 35000.0, "PREMIUM"),
		vo.NewFoodNutrient("p3", "Cá hồi phi lê", "PROTEIN", 208, 20, 0, 13, nil, "SALMON", "", false, 45000.0, "PREMIUM"),
		vo.NewFoodNutrient("p4", "Trứng gà tươi", "PROTEIN", 155, 13, 1.1, 11, []string{"EGG"}, "EGG", "", false, 6000.0, "BUDGET"),
		vo.NewFoodNutrient("p5", "Đậu hũ trắng", "PROTEIN", 76, 8, 1.9, 4.8, []string{"SOY"}, "PLANT", "", false, 5000.0, "BUDGET"),
		vo.NewFoodNutrient("p6", "Thịt heo nạc", "PROTEIN", 143, 21, 0, 6.2, nil, "PORK", "", false, 16000.0, "STANDARD"),
		vo.NewFoodNutrient("p7", "Tôm sú tươi", "PROTEIN", 99, 24, 0.2, 0.3, []string{"SHELLFISH"}, "SEAFOOD", "", false, 38000.0, "PREMIUM"),

		// Carbs
		vo.NewFoodNutrient("c1", "Cơm trắng gạo tẻ", "CARB", 130, 2.7, 28, 0.3, nil, "", "RICE", false, 3500.0, "BUDGET"),
		vo.NewFoodNutrient("c2", "Cơm gạo lứt", "CARB", 112, 2.6, 23, 0.9, nil, "", "RICE", false, 8000.0, "STANDARD"),
		vo.NewFoodNutrient("c3", "Khoai lang luộc", "CARB", 90, 2.0, 21, 0.1, nil, "", "SWEET_POTATO", false, 4500.0, "BUDGET"),
		vo.NewFoodNutrient("c4", "Yến mạch cán dẹt", "CARB", 389, 16.9, 66, 6.9, nil, "", "OATS", false, 12000.0, "STANDARD"),
		vo.NewFoodNutrient("c5", "Bún tươi", "CARB", 110, 1.7, 25.7, 0, nil, "", "NOODLE", false, 3500.0, "BUDGET"),
		vo.NewFoodNutrient("c6", "Bánh mì nguyên cám", "CARB", 247, 13, 41, 3.4, []string{"GLUTEN"}, "", "BREAD", false, 10000.0, "STANDARD"),

		// Veggies
		vo.NewFoodNutrient("v1", "Bông cải xanh", "VEGGIE", 34, 2.8, 7.0, 0.4, nil, "", "", false, 9000.0, "STANDARD"),
		vo.NewFoodNutrient("v2", "Rau muống luộc", "VEGGIE", 19, 3.0, 2.1, 0.2, nil, "", "", false, 3500.0, "BUDGET"),
		vo.NewFoodNutrient("v3", "Cải thìa xào tỏi", "VEGGIE", 25, 1.5, 3.0, 0.8, nil, "", "", false, 4000.0, "BUDGET"),
		vo.NewFoodNutrient("v4", "Nấm đùi gà", "VEGGIE", 35, 3.1, 5.0, 0.3, nil, "", "", false, 12000.0, "STANDARD"),

		// Snacks / Fat
		vo.NewFoodNutrient("s1", "Sữa chua không đường", "SNACK", 61, 3.5, 4.7, 3.3, []string{"DAIRY"}, "", "", false, 7000.0, "BUDGET"),
		vo.NewFoodNutrient("s2", "Hạt hạnh nhân", "SNACK", 579, 21, 22, 50, []string{"TREE_NUT"}, "", "", false, 25000.0, "PREMIUM"),
		vo.NewFoodNutrient("s3", "Chuối chín", "SNACK", 89, 1.1, 23, 0.3, nil, "", "", false, 4000.0, "BUDGET"),
	}
}

func TestCandidateSelector_HardAllergenFilter(t *testing.T) {
	selector := NewCandidateSelector()
	catalog := makeTestCatalog()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	// User dị ứng GLUTEN và DAIRY
	criteria := SelectionCriteria{
		Catalog:          catalog,
		UserRestrictions: []string{"GLUTEN", "DAIRY"},
		UserID:           "user-allergy",
		PlanDate:         now,
		NutritionGoal:    vo.GoalMuscleGain,
	}

	candidates := selector.SelectCandidates(&criteria)
	for _, c := range candidates {
		if c.Name() == "Bánh mì nguyên cám" {
			t.Errorf("Allergen violation: Bánh mì nguyên cám must be strictly excluded")
		}
		if c.Name() == "Sữa chua không đường" {
			t.Errorf("Allergen violation: Sữa chua không đường must be strictly excluded")
		}
	}
}

func TestCandidateSelector_SoftLockoutExponentialDecay(t *testing.T) {
	selector := NewCandidateSelector()
	planDate := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	chickenItem := vo.NewFoodNutrient(
		"p1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000.0, "BUDGET",
	)

	// Trường hợp 1: Mới ăn hôm qua (1 ngày trước) -> Phạt cao
	recentConsumptions1Day := []vo.HistoricalConsumption{
		{
			MealName:    "Ức gà áp chảo",
			Ingredients: []string{"Ức gà tươi"},
			ConsumedAt:  planDate.Add(-24 * time.Hour), // 1 ngày trước
		},
	}
	penalty1Day := selector.CalculateRecencyPenalty(&chickenItem, recentConsumptions1Day, planDate)

	// Trường hợp 2: Ăn từ 5 ngày trước -> Phạt rất thấp
	recentConsumptions5Days := []vo.HistoricalConsumption{
		{
			MealName:    "Ức gà áp chảo",
			Ingredients: []string{"Ức gà tươi"},
			ConsumedAt:  planDate.Add(-5 * 24 * time.Hour), // 5 ngày trước
		},
	}
	penalty5Days := selector.CalculateRecencyPenalty(&chickenItem, recentConsumptions5Days, planDate)

	if penalty1Day <= penalty5Days {
		t.Fatalf("expected penalty after 1 day (%f) to be higher than 5 days (%f)", penalty1Day, penalty5Days)
	}

	// Xác nhận penalty1Day ~ e^{-1/2} = e^{-0.5} ~ 0.606
	if penalty1Day < 0.55 || penalty1Day > 0.65 {
		t.Errorf("expected penalty1Day around 0.606, got %f", penalty1Day)
	}
}

func TestCandidateSelector_BudgetPenalty(t *testing.T) {
	selector := NewCandidateSelector()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	catalog := makeTestCatalog()

	// 1. Kiểm tra khi không cấu hình target budget (rỗng): Phạt chi phí phải bằng 0.0 (không thiên vị/phạt bất kỳ tier nào)
	premiumItem := vo.NewFoodNutrient("p2", "Thịt bò thăn", "PROTEIN", 250, 26, 0, 15, nil, "BEEF", "", false, 35000.0, "PREMIUM")
	if penalty := calculateCostPenalty(&premiumItem, ""); penalty != 0.0 {
		t.Fatalf("expected cost penalty = 0.0 when targetBudget is empty, got %f", penalty)
	}

	// 2. Ngân sách BUDGET -> Món BUDGET (Ức gà, Cơm trắng) được ưu tiên hơn PREMIUM (Thịt bò, Cá hồi)
	criteriaBudget := SelectionCriteria{
		Catalog:          catalog,
		UserID:           "user-frugal",
		PlanDate:         now,
		NutritionGoal:    vo.GoalMuscleGain,
		TargetBudgetTier: "BUDGET",
	}

	selected := selector.SelectCandidates(&criteriaBudget)

	// Kiểm tra xem Ức gà (BUDGET) có được chọn không
	var hasChicken bool
	for _, item := range selected {
		if item.Name() == "Ức gà tươi" {
			hasChicken = true
		}
	}
	if !hasChicken {
		t.Errorf("expected Ức gà tươi (BUDGET) to be prioritized under BUDGET tier")
	}
}

func TestCandidateSelector_PersonalizedPreference(t *testing.T) {
	selector := NewCandidateSelector()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	catalog := makeTestCatalog()

	// Người dùng thích ăn Cơm trắng
	criteria := SelectionCriteria{
		Catalog:         catalog,
		UserID:          "user-rice-lover",
		PlanDate:        now,
		NutritionGoal:   vo.GoalMaintenance,
		UserPreferences: []string{"Cơm trắng", "Ức gà"},
	}

	selected := selector.SelectCandidates(&criteria)
	var foundRice bool
	for _, item := range selected {
		if item.Name() == "Cơm trắng gạo tẻ" {
			foundRice = true
		}
	}

	if !foundRice {
		t.Errorf("expected Cơm trắng gạo tẻ to be selected due to user preference fit")
	}
}

func TestCandidateSelector_MinimumEssentialThreshold(t *testing.T) {
	selector := NewCandidateSelector()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	p := vo.NewFoodNutrient("p1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET")
	c := vo.NewFoodNutrient("c1", "Cơm trắng", "CARB", 130, 2.7, 28, 0.3, nil, "", "RICE", false, 3500, "BUDGET")
	v := vo.NewFoodNutrient("v1", "Bông cải", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false, 9000, "STANDARD")

	// Case 1: Catalog chỉ có 1 nguyên liệu (Protein) -> Phải từ chối (nil)
	res1 := selector.SelectCandidates(&SelectionCriteria{
		Catalog:  []vo.FoodNutrient{p},
		UserID:   "u1",
		PlanDate: now,
	})
	if res1 != nil {
		t.Errorf("expected nil when catalog has only 1 ingredient, got %d items", len(res1))
	}

	// Case 2: Catalog chỉ có 2 nguyên liệu (Protein + Carb, thiếu Veggie) -> Phải từ chối (nil)
	res2 := selector.SelectCandidates(&SelectionCriteria{
		Catalog:  []vo.FoodNutrient{p, c},
		UserID:   "u1",
		PlanDate: now,
	})
	if res2 != nil {
		t.Errorf("expected nil when catalog lacks Veggie pillar, got %d items", len(res2))
	}

	// Case 3: Catalog có đủ 3 nhóm thiết yếu (1 Protein + 1 Carb + 1 Veggie) -> Chấp nhận
	res3 := selector.SelectCandidates(&SelectionCriteria{
		Catalog:  []vo.FoodNutrient{p, c, v},
		UserID:   "u1",
		PlanDate: now,
	})
	if len(res3) != 3 {
		t.Errorf("expected 3 items when all 3 pillars are present, got %d", len(res3))
	}
}
