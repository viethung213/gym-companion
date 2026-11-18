package service

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// SelectPantryCandidates phân tách rõ ràng 2 danh sách nguyên liệu:
//  1. mandatory: Toàn bộ nguyên liệu do người dùng nhập từ tủ lạnh (BẮT BUỘC AI phải dùng, cấm thay thế).
//  2. supplementary: Các món an toàn lấy ngẫu nhiên từ DB cho các nhóm dinh dưỡng còn thiếu (GỢI Ý BỔ SUNG,
//     lấy theo cơ chế giống tool fetch_food_catalog).
func SelectPantryCandidates(
	pantryItems []vo.FoodNutrient,
	activeCatalog []vo.FoodNutrient,
	userRestrictions []string,
	userID string,
	planDate time.Time,
) (mandatory, supplementary []vo.FoodNutrient) {
	mandatory = pantryItems
	if len(activeCatalog) == 0 {
		return mandatory, nil
	}

	// 1. Kiểm tra các nhóm macro đã có trong nguyên liệu của người dùng
	var hasProtein, hasCarb, hasVeggie bool
	existingIDs := make(map[string]bool)
	existingNames := make(map[string]bool)

	for i := range pantryItems {
		if pantryItems[i].ID() != "" {
			existingIDs[pantryItems[i].ID()] = true
		}
		nameLower := strings.ToLower(strings.TrimSpace(pantryItems[i].Name()))
		if nameLower != "" {
			existingNames[nameLower] = true
		}

		cat := strings.ToUpper(strings.TrimSpace(pantryItems[i].Category()))
		switch cat {
		case "PROTEIN":
			hasProtein = true
		case "CARB":
			hasCarb = true
		case "VEGGIE":
			hasVeggie = true
		}
	}

	// Nếu người dùng đã cung cấp đủ cả 3 nhóm Protein + Carb + Veggie, không cần bổ sung DB
	if hasProtein && hasCarb && hasVeggie {
		return mandatory, nil
	}

	// 2. Lấy nguyên liệu an toàn ngẫu nhiên từ DB cho các nhóm còn thiếu (chỉ lọc dị ứng)
	safeCatalog := FilterAllergensOnly(activeCatalog, userRestrictions)
	proteins, carbs, veggies, _, _ := CategorizeFoods(safeCatalog)

	supplementary = make([]vo.FoodNutrient, 0)

	addCandidates := func(candidates []vo.FoodNutrient, count int, groupKey string) {
		filtered := make([]vo.FoodNutrient, 0, len(candidates))
		for i := range candidates {
			c := &candidates[i]
			nameLower := strings.ToLower(strings.TrimSpace(c.Name()))
			if !existingIDs[c.ID()] && !existingNames[nameLower] {
				filtered = append(filtered, *c)
			}
		}
		rng := NewDeterministicRNG(userID+"_pantry_"+groupKey, planDate)
		picked := SamplePaged(filtered, count, 1, rng)
		for i := range picked {
			p := &picked[i]
			existingIDs[p.ID()] = true
			existingNames[strings.ToLower(strings.TrimSpace(p.Name()))] = true
			supplementary = append(supplementary, *p)
		}
	}

	if !hasProtein {
		addCandidates(proteins, 4, "PROTEIN")
	}
	if !hasCarb {
		addCandidates(carbs, 4, "CARB")
	}
	if !hasVeggie {
		addCandidates(veggies, 4, "VEGGIE")
	}

	return mandatory, supplementary
}
