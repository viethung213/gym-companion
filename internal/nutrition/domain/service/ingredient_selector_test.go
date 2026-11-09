package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func createMockCatalog() []vo.FoodNutrient {
	items := make([]vo.FoodNutrient, 0)

	// 20 món Protein (Gia cầm, Thịt đỏ, Hải sản, Trứng/Đậu)
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("pro-poultry-%d", i), fmt.Sprintf("Ức gà số %d", i), "PROTEIN",
			165, 31, 0, 3.6, nil, "CHICKEN", "", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("pro-beef-%d", i), fmt.Sprintf("Thăn bò số %d", i), "PROTEIN",
			250, 26, 0, 15, nil, "BEEF", "", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("pro-seafood-%d", i), fmt.Sprintf("Cá hồi số %d", i), "PROTEIN",
			200, 20, 0, 12, []string{"SEAFOOD"}, "FISH", "", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("pro-egg-%d", i), fmt.Sprintf("Trứng gà số %d", i), "PROTEIN",
			155, 13, 1.1, 11, []string{"EGG"}, "EGG", "", false,
		))
	}

	// 20 món Carb (Cơm, Khoai, Yến mạch, Bún)
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("carb-rice-%d", i), fmt.Sprintf("Cơm gạo lứt số %d", i), "CARB",
			110, 2.6, 23, 0.9, nil, "", "RICE", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("carb-potato-%d", i), fmt.Sprintf("Khoai lang số %d", i), "CARB",
			90, 2, 21, 0.1, nil, "", "POTATO", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("carb-oats-%d", i), fmt.Sprintf("Yến mạch số %d", i), "CARB",
			389, 16.9, 66.3, 6.9, []string{"GLUTEN"}, "", "OATS", false,
		))
	}
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("carb-noodle-%d", i), fmt.Sprintf("Bún gạo lứt số %d", i), "CARB",
			110, 2, 24, 0.5, nil, "", "NOODLE", false,
		))
	}

	// 15 món Veggie
	for i := 1; i <= 15; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("veg-%d", i), fmt.Sprintf("Bông cải xanh số %d", i), "VEGGIE",
			34, 2.8, 7, 0.4, nil, "", "", false,
		))
	}

	// 10 món Snack
	for i := 1; i <= 10; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("snack-%d", i), fmt.Sprintf("Hạnh nhân số %d", i), "SNACK",
			579, 21, 22, 50, []string{"NUTS"}, "", "", false,
		))
	}

	// 5 món NutiFood (phục vụ bữa phụ SNACK tiện lợi)
	for i := 1; i <= 5; i++ {
		items = append(items, vo.NewFoodNutrient(
			fmt.Sprintf("nutifood-%d", i), fmt.Sprintf("Sữa NutiFood số %d", i), "NUTIFOOD",
			150, 15, 10, 5, nil, "", "", true,
		))
	}

	return items
}

func TestSelectDiverseCandidates_FiltersAllergensAndLockout(t *testing.T) {
	catalog := createMockCatalog()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-test-123"

	// Khóa món bò
	lockouts := vo.NewLockoutRegistry(nil).ApplyLockout(
		vo.LockoutTypeProtein, "Thăn bò số 1", 7*24*time.Hour, now,
	)

	// Dị ứng hải sản
	restrictions := []string{"SEAFOOD"}

	result := SelectDiverseCandidates(catalog, lockouts, restrictions, userID, now)

	if len(result) == 0 {
		t.Fatalf("expected result not empty, got 0")
	}

	for _, item := range result {
		// Kiểm tra không có hải sản
		for _, tag := range item.AllergenTags() {
			if strings.EqualFold(tag, "SEAFOOD") {
				t.Errorf("found allergen SEAFOOD in result: %s", item.Name())
			}
		}
		// Kiểm tra không có món bị khóa
		if item.Name() == "Thăn bò số 1" {
			t.Errorf("found locked item 'Thăn bò số 1' in result")
		}
		// Kiểm tra NutiFood nếu có xuất hiện thì phải thuộc nhóm SNACK tiện lợi
		if item.IsNutiFoodProduct() && item.Category() == "PROTEIN" {
			t.Errorf("NutiFood product should not be classified as main cooked PROTEIN: %s", item.Name())
		}
	}
}

func TestDefaultCandidateConfig_Returns21Items(t *testing.T) {
	cfg := DefaultCandidateConfig()
	total := cfg.TargetProteinCount + cfg.TargetCarbCount + cfg.TargetVeggieCount + cfg.TargetSnackCount

	if total != 21 {
		t.Errorf("got total %d candidates, want 21", total)
	}
	if cfg.TargetProteinCount != 6 || cfg.TargetCarbCount != 6 || cfg.TargetVeggieCount != 6 || cfg.TargetSnackCount != 3 {
		t.Errorf("unexpected distribution: %+v", cfg)
	}

	catalog := createMockCatalog()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-test-21"
	lockouts := vo.NewLockoutRegistry(nil)

	result := SelectDiverseCandidates(catalog, lockouts, nil, userID, now)
	if len(result) != 21 {
		t.Fatalf("expected 21 items with default config, got %d", len(result))
	}
}

func TestSelectDiverseCandidates_StratifiedQuotas(t *testing.T) {
	catalog := createMockCatalog()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-test-123"
	lockouts := vo.NewLockoutRegistry(nil)

	cfg := CandidateSelectionConfig{
		TargetProteinCount: 10,
		TargetCarbCount:    10,
		TargetVeggieCount:  8,
		TargetSnackCount:   4,
	}

	result := SelectDiverseCandidates(catalog, lockouts, nil, userID, now, cfg)

	if len(result) != 32 {
		t.Fatalf("expected 32 items, got %d", len(result))
	}

	var proCount, carbCount, vegCount, snackCount int
	for _, f := range result {
		cat := strings.ToUpper(f.Category())
		if f.IsNutiFoodProduct() || cat == "NUTIFOOD" || cat == "SNACK" {
			snackCount++
		} else if cat == "PROTEIN" {
			proCount++
		} else if cat == "CARB" {
			carbCount++
		} else if cat == "VEGGIE" {
			vegCount++
		}
	}

	if proCount != 10 {
		t.Errorf("expected 10 proteins, got %d", proCount)
	}
	if carbCount != 10 {
		t.Errorf("expected 10 carbs, got %d", carbCount)
	}
	if vegCount != 8 {
		t.Errorf("expected 8 veggies, got %d", vegCount)
	}
	if snackCount != 4 {
		t.Errorf("expected 4 snacks, got %d", snackCount)
	}
}

func TestSelectDiverseCandidates_DeterministicDailyRotation(t *testing.T) {
	catalog := createMockCatalog()
	userID := "user-1"
	lockouts := vo.NewLockoutRegistry(nil)

	day1 := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

	// Lần 1 và Lần 2 trong cùng ngày 1 -> Kết quả phải giống hệt nhau 100%
	res1A := SelectDiverseCandidates(catalog, lockouts, nil, userID, day1)
	res1B := SelectDiverseCandidates(catalog, lockouts, nil, userID, day1)

	if len(res1A) != len(res1B) {
		t.Fatalf("different lengths for identical day")
	}
	for i := range res1A {
		if res1A[i].ID() != res1B[i].ID() {
			t.Errorf("expected same item at idx %d for same day, got %s vs %s", i, res1A[i].ID(), res1B[i].ID())
		}
	}

	// Ngày 2 -> Xoay vòng món ăn khác biệt so với Ngày 1
	res2 := SelectDiverseCandidates(catalog, lockouts, nil, userID, day2)
	sameCount := 0
	for i := range res1A {
		for j := range res2 {
			if res1A[i].ID() == res2[j].ID() {
				sameCount++
				break
			}
		}
	}

	// Không được trùng lặp 100% giữa 2 ngày khác nhau
	if sameCount == len(res1A) {
		t.Errorf("expected different rotation between day 1 and day 2, but got 100%% identical")
	}
}

func TestSelectDiverseCandidates_SmallCatalog(t *testing.T) {
	// Catalog chỉ có 3 món nhỏ hơn target quota
	small := []vo.FoodNutrient{
		vo.NewFoodNutrient("1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false),
		vo.NewFoodNutrient("2", "Gạo lứt", "CARB", 110, 2.6, 23, 0.9, nil, "", "", false),
		vo.NewFoodNutrient("3", "Bông cải", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false),
	}
	lockouts := vo.NewLockoutRegistry(nil)
	now := time.Now()

	result := SelectDiverseCandidates(small, lockouts, nil, "user-1", now)
	if len(result) != 3 {
		t.Fatalf("expected all 3 items to be returned, got %d", len(result))
	}
}

func TestSelectPantryCandidates_OnlyProteinProvided(t *testing.T) {
	catalog := createMockCatalog()
	lockouts := vo.NewLockoutRegistry(nil)
	planDate := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-pantry-1"

	// Người dùng chỉ có 1 nguyên liệu duy nhất: Ức gà
	userItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-1", "Ức gà hấp", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false),
	}

	mandatory, supplementary := SelectPantryCandidates(userItems, catalog, lockouts, nil, userID, planDate)

	if len(mandatory) != 1 || mandatory[0].Name() != "Ức gà hấp" {
		t.Fatalf("expected mandatory to be user's original ingredient, got %v", mandatory)
	}

	// Kiểm tra đã có cả Carb và Veggie trong supplementary
	var hasCarb, hasVeggie bool
	for _, item := range supplementary {
		switch strings.ToUpper(strings.TrimSpace(item.Category())) {
		case "CARB":
			hasCarb = true
		case "VEGGIE":
			hasVeggie = true
		}
	}

	if !hasCarb {
		t.Errorf("expected CARB candidates to be supplemented")
	}
	if !hasVeggie {
		t.Errorf("expected VEGGIE candidates to be supplemented")
	}
}

func TestSelectPantryCandidates_AllGroupsAlreadyProvided(t *testing.T) {
	catalog := createMockCatalog()
	lockouts := vo.NewLockoutRegistry(nil)
	planDate := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-pantry-2"

	// Người dùng đã có đủ cả 3 nhóm
	userItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false),
		vo.NewFoodNutrient("p-2", "Khoai lang luộc", "CARB", 90, 2, 21, 0.1, nil, "", "", false),
		vo.NewFoodNutrient("p-3", "Bông cải xanh", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false),
	}

	mandatory, supplementary := SelectPantryCandidates(userItems, catalog, lockouts, nil, userID, planDate)

	// Không cần bổ sung thêm nếu đã đủ
	if len(mandatory) != 3 {
		t.Errorf("expected exactly 3 mandatory items, got %d", len(mandatory))
	}
	if len(supplementary) != 0 {
		t.Errorf("expected 0 supplementary items when all groups provided, got %d", len(supplementary))
	}
}

func TestSelectPantryCandidates_FiltersAllergensAndLockouts(t *testing.T) {
	catalog := createMockCatalog()
	planDate := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-pantry-3"

	// Khóa GLUTEN (loại yến mạch)
	restrictions := []string{"GLUTEN"}
	lockouts := vo.NewLockoutRegistry(nil)

	userItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-1", "Trứng chiên", "PROTEIN", 155, 13, 1.1, 11, nil, "", "", false),
	}

	_, supplementary := SelectPantryCandidates(userItems, catalog, lockouts, restrictions, userID, planDate)

	for _, item := range supplementary {
		for _, tag := range item.AllergenTags() {
			if strings.EqualFold(tag, "GLUTEN") {
				t.Errorf("supplemented item %s contains restricted allergen GLUTEN", item.Name())
			}
		}
	}
}

func TestSelectPantryCandidates_Segregation(t *testing.T) {
	catalog := createMockCatalog()
	planDate := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	userID := "user-pantry-4"
	lockouts := vo.NewLockoutRegistry(nil)

	// Người dùng gửi nguyên liệu từ tủ lạnh (có thể là nguyên liệu tự do)
	userItems := []vo.FoodNutrient{
		vo.NewFoodNutrient("p-unknown-1", "Khổ qua rừng", "INGREDIENT", 0, 0, 0, 0, nil, "", "", false),
	}

	mandatory, supplementary := SelectPantryCandidates(userItems, catalog, lockouts, nil, userID, planDate)

	// 1. mandatory chỉ chứa nguyên liệu gốc của người dùng
	if len(mandatory) != 1 || mandatory[0].Name() != "Khổ qua rừng" {
		t.Fatalf("expected mandatory to contain user's item, got %v", mandatory)
	}

	// 2. supplementary chứa các ứng viên DB được bốc sẵn ngẫu nhiên cho các nhóm còn thiếu
	if len(supplementary) == 0 {
		t.Fatalf("expected supplementary candidates to be pre-selected from DB for missing groups")
	}

	// Kiểm tra các ứng viên trong supplementary không bị trùng ID với mandatory
	for _, supp := range supplementary {
		if supp.ID() == mandatory[0].ID() {
			t.Errorf("supplementary item %s overlaps with mandatory item", supp.Name())
		}
	}
}

func TestSamplePaged_PaginationAndOffset(t *testing.T) {
	catalog := createMockCatalog()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	// Mỗi request khởi tạo Deterministic RNG từ seed (UserID + Date)
	rng1 := NewDeterministicRNG("test-user", now)
	page1 := SamplePaged(catalog, 6, 1, rng1)
	if len(page1) != 6 {
		t.Fatalf("expected page1 to have 6 items, got %d", len(page1))
	}

	rng2 := NewDeterministicRNG("test-user", now)
	page2 := SamplePaged(catalog, 6, 2, rng2)
	if len(page2) != 6 {
		t.Fatalf("expected page2 to have 6 items, got %d", len(page2))
	}

	// Verify items between page 1 and page 2 do not overlap
	p1IDs := make(map[string]bool)
	for _, it := range page1 {
		p1IDs[it.ID()] = true
	}
	overlap := 0
	for _, it := range page2 {
		if p1IDs[it.ID()] {
			overlap++
		}
	}
	if overlap > 0 {
		t.Errorf("expected no overlap between consecutive page 1 and page 2, got %d overlapping items", overlap)
	}
}
