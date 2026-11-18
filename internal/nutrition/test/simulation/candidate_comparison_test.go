package simulation_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// buildRealisticCatalog khởi tạo danh mục 20 nguyên liệu thực tế phổ biến tại Việt Nam
// đầy đủ phân khúc giá (BUDGET, STANDARD, PREMIUM) và macro.
func buildRealisticCatalog() []vo.FoodNutrient {
	return []vo.FoodNutrient{
		// === PROTEIN ===
		vo.NewFoodNutrient("p1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false, 13000, "BUDGET"),
		vo.NewFoodNutrient("p2", "Thịt heo nạc", "PROTEIN", 143, 21, 0, 6.2, nil, "PORK", "", false, 16000, "STANDARD"),
		vo.NewFoodNutrient("p3", "Trứng gà tươi", "PROTEIN", 155, 13, 1.1, 11, []string{"EGG"}, "EGG", "", false, 6000, "BUDGET"),
		vo.NewFoodNutrient("p4", "Thịt bò thăn", "PROTEIN", 250, 26, 0, 15, nil, "BEEF", "", false, 35000, "PREMIUM"),
		vo.NewFoodNutrient("p5", "Cá hồi phi lê", "PROTEIN", 208, 20, 0, 13, nil, "SALMON", "", false, 45000, "PREMIUM"),
		vo.NewFoodNutrient("p6", "Đậu hũ trắng", "PROTEIN", 76, 8, 1.9, 4.8, []string{"SOY"}, "PLANT", "", false, 5000, "BUDGET"),
		vo.NewFoodNutrient("p7", "Tôm sú tươi", "PROTEIN", 99, 24, 0.2, 0.3, []string{"SHELLFISH"}, "SEAFOOD", "", false, 38000, "PREMIUM"),

		// === CARB ===
		vo.NewFoodNutrient("c1", "Cơm trắng gạo tẻ", "CARB", 130, 2.7, 28, 0.3, nil, "", "RICE", false, 3500, "BUDGET"),
		vo.NewFoodNutrient("c2", "Cơm gạo lứt", "CARB", 112, 2.6, 23, 0.9, nil, "", "RICE", false, 8000, "STANDARD"),
		vo.NewFoodNutrient("c3", "Khoai lang luộc", "CARB", 90, 2.0, 21, 0.1, nil, "", "SWEET_POTATO", false, 4500, "BUDGET"),
		vo.NewFoodNutrient("c4", "Yến mạch cán dẹt", "CARB", 389, 16.9, 66, 6.9, nil, "", "OATS", false, 12000, "STANDARD"),
		vo.NewFoodNutrient("c5", "Bún tươi", "CARB", 110, 1.7, 25.7, 0, nil, "", "NOODLE", false, 3500, "BUDGET"),
		vo.NewFoodNutrient("c6", "Bánh mì nguyên cám", "CARB", 247, 13, 41, 3.4, []string{"GLUTEN"}, "", "BREAD", false, 10000, "STANDARD"),

		// === VEGGIE ===
		vo.NewFoodNutrient("v1", "Bông cải xanh", "VEGGIE", 34, 2.8, 7.0, 0.4, nil, "", "", false, 9000, "STANDARD"),
		vo.NewFoodNutrient("v2", "Rau muống luộc", "VEGGIE", 19, 3.0, 2.1, 0.2, nil, "", "", false, 3500, "BUDGET"),
		vo.NewFoodNutrient("v3", "Cải thìa xào tỏi", "VEGGIE", 25, 1.5, 3.0, 0.8, nil, "", "", false, 4000, "BUDGET"),
		vo.NewFoodNutrient("v4", "Nấm đùi gà", "VEGGIE", 35, 3.1, 5.0, 0.3, nil, "", "", false, 12000, "STANDARD"),

		// === SNACK / FAT ===
		vo.NewFoodNutrient("s1", "Sữa chua không đường", "SNACK", 61, 3.5, 4.7, 3.3, []string{"DAIRY"}, "", "", false, 7000, "BUDGET"),
		vo.NewFoodNutrient("s2", "Chuối chín", "SNACK", 89, 1.1, 23, 0.3, nil, "", "", false, 4000, "BUDGET"),
		vo.NewFoodNutrient("s3", "Hạt hạnh nhân", "SNACK", 579, 21, 22, 50, []string{"TREE_NUT"}, "", "", false, 25000, "PREMIUM"),
	}
}

func containsFood(candidates []vo.FoodNutrient, foodName string) bool {
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c.Name()), strings.ToLower(foodName)) {
			return true
		}
	}
	return false
}

func pickCandidate(candidates []vo.FoodNutrient, category, preferredKeyword string) vo.FoodNutrient {
	var fallback vo.FoodNutrient
	for _, c := range candidates {
		if strings.EqualFold(c.Category(), category) {
			if fallback.Name() == "" {
				fallback = c
			}
			if preferredKeyword != "" && strings.Contains(strings.ToLower(c.Name()), strings.ToLower(preferredKeyword)) {
				return c
			}
		}
	}
	return fallback
}

func formatFoodNames(items []vo.FoodNutrient) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name())
	}
	return strings.Join(names, ", ")
}

// TestSimulation_7Days_ScoringCandidateSelector mô phỏng 7 ngày liên tiếp của người dùng
// thích ăn Cơm trắng và Ức gà, ngân sách BUDGET theo cơ chế tính điểm mềm.
func TestSimulation_7Days_ScoringCandidateSelector(t *testing.T) {
	catalog := buildRealisticCatalog()
	startDate := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	userID := "user-simulation-01"

	// Cấu hình kịch bản người dùng:
	userPreferences := []string{"Cơm trắng gạo tẻ", "Ức gà tươi"}
	userRestrictions := []string{"SHELLFISH"} // Dị ứng tôm
	targetBudget := "BUDGET"

	scoringSelector := service.NewCandidateSelector()
	scoringHistory := make([]vo.HistoricalConsumption, 0)

	fmt.Println("\n=========================================================================================")
	fmt.Println("🚀 BẮT ĐẦU MÔ PHỎNG 7 NGÀY CHỌN NGUYÊN LIỆU ĐIỂM MỀM (SOFT SCORING)")
	fmt.Printf("   User: %s | Sở thích: %v | Ngân sách: %s | Dị ứng: %v\n", userID, userPreferences, targetBudget, userRestrictions)
	fmt.Println("=========================================================================================")

	for day := 1; day <= 7; day++ {
		currentDate := startDate.AddDate(0, 0, day-1)
		dayName := currentDate.Weekday().String()

		fmt.Printf("\n📅 [NGÀY %d - %s (%s)]\n", day, dayName, currentDate.Format("2006-01-02"))

		scoringCriteria := service.SelectionCriteria{
			Catalog:            catalog,
			UserRestrictions:   userRestrictions,
			UserID:             userID,
			PlanDate:           currentDate,
			NutritionGoal:      vo.GoalMuscleGain,
			UserPreferences:    userPreferences,
			TargetBudgetTier:   targetBudget,
			RecentConsumptions: scoringHistory,
		}
		scoringCandidates := scoringSelector.SelectCandidates(&scoringCriteria)

		hasScoringRice := containsFood(scoringCandidates, "Cơm trắng")
		hasScoringChicken := containsFood(scoringCandidates, "Ức gà")

		fmt.Println("  🟢 KẾT QUẢ CHỌN NGUYÊN LIỆU (Soft Scoring):")
		fmt.Printf("     - Có Cơm trắng? %v | Có Ức gà? %v\n", hasScoringRice, hasScoringChicken)
		fmt.Printf("     - Danh sách: %s\n", formatFoodNames(scoringCandidates))

		// Ghi nhận tiêu thụ ngày 1
		if day == 1 {
			consumedProtein := pickCandidate(scoringCandidates, "PROTEIN", "Ức gà")
			consumedCarb := pickCandidate(scoringCandidates, "CARB", "Cơm")

			scoringHistory = append(scoringHistory,
				vo.HistoricalConsumption{
					MealName:    consumedProtein.Name(),
					Ingredients: []string{consumedProtein.Name()},
					ConsumedAt:  currentDate,
				},
				vo.HistoricalConsumption{
					MealName:    consumedCarb.Name(),
					Ingredients: []string{consumedCarb.Name()},
					ConsumedAt:  currentDate,
				},
			)
			fmt.Printf("  📝 [GHI NHẬN HÔM NAY]: Đạm: %s (%s) | Tinh bột: %s (%s)\n",
				consumedProtein.Name(), consumedProtein.ProteinSource(), consumedCarb.Name(), consumedCarb.CarbSource())
		}

		// Kiểm chứng dị ứng: Không bao giờ có Tôm sú (SHELLFISH)
		if containsFood(scoringCandidates, "Tôm sú") {
			t.Errorf("Ngày %d: Vi phạm dị ứng Tôm sú", day)
		}
	}
}
