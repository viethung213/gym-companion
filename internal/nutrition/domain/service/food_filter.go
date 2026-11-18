package service

import (
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// FilterAllergensOnly loại bỏ các món ăn vi phạm danh sách dị ứng của người dùng.
func FilterAllergensOnly(catalog []vo.FoodNutrient, userRestrictions []string) []vo.FoodNutrient {
	if len(userRestrictions) == 0 {
		return catalog
	}

	restrictionMap := make(map[string]bool, len(userRestrictions))
	for _, r := range userRestrictions {
		norm := strings.ToUpper(strings.TrimSpace(r))
		if norm != "" {
			restrictionMap[norm] = true
		}
	}

	safe := make([]vo.FoodNutrient, 0, len(catalog))
itemLoop:
	for i := range catalog {
		item := &catalog[i]

		// 1. Kiểm tra thẻ dị ứng (AllergenTags)
		for _, tag := range item.AllergenTags() {
			if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
				continue itemLoop
			}
		}

		// 2. Kiểm tra tên nguyên liệu
		nameUpper := strings.ToUpper(strings.TrimSpace(item.Name()))
		for r := range restrictionMap {
			if strings.Contains(nameUpper, r) {
				continue itemLoop
			}
		}

		safe = append(safe, *item)
	}
	return safe
}

// CategorizeFoods phân chia các món ăn an toàn thành các nhóm chính:
// - proteins: nguồn đạm chính
// - carbs: nguồn tinh bột chính
// - veggies: rau xanh, nấm, củ quả
// - snacks: bữa phụ, sữa chua, sản phẩm NutiFood, hạt
// - others: các nguyên liệu chưa thuộc 4 nhóm trên
func CategorizeFoods(foods []vo.FoodNutrient) (proteins, carbs, veggies, snacks, others []vo.FoodNutrient) {
	for i := range foods {
		f := foods[i]
		cat := strings.ToUpper(strings.TrimSpace(f.Category()))

		switch {
		case f.IsNutiFoodProduct() || cat == "NUTIFOOD" || cat == "SNACK" || cat == "FAT" || cat == "DAIRY" || cat == "FRUIT":
			snacks = append(snacks, f)
		case cat == "PROTEIN":
			proteins = append(proteins, f)
		case cat == "CARB":
			carbs = append(carbs, f)
		case cat == "VEGGIE" || cat == "VEGETABLE":
			veggies = append(veggies, f)
		case strings.Contains(cat, "MEAT") || strings.Contains(cat, "FISH") || f.ProteinPer100g() >= 18:
			proteins = append(proteins, f)
		case strings.Contains(cat, "GRAIN") || f.CarbsPer100g() >= 20:
			carbs = append(carbs, f)
		case strings.Contains(cat, "SALAD") || strings.Contains(cat, "MUSHROOM") || strings.Contains(cat, "VEG"):
			veggies = append(veggies, f)
		default:
			others = append(others, f)
		}
	}
	return
}
