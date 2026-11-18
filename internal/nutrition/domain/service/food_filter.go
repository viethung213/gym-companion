package service

import (
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
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

// ValidatePantryIngredientsAgainstAllergens kiểm tra các nguyên liệu người dùng gửi lên so với danh sách kiêng cữ/dị ứng.
// Nếu phát hiện nguyên liệu trùng tên hoặc chứa chất dị ứng, hàm trả về ErrAllergenConflict kèm thông điệp chi tiết.
func ValidatePantryIngredientsAgainstAllergens(
	userIngredients []string,
	activeCatalog []vo.FoodNutrient,
	userRestrictions []string,
) error {
	if len(userRestrictions) == 0 || len(userIngredients) == 0 {
		return nil
	}

	restrictionMap := make(map[string]bool, len(userRestrictions))
	for _, r := range userRestrictions {
		norm := strings.ToUpper(strings.TrimSpace(r))
		if norm != "" {
			restrictionMap[norm] = true
		}
	}

	for _, ing := range userIngredients {
		trimmed := strings.TrimSpace(ing)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)

		// 1. Kiểm tra trực tiếp tên nguyên liệu với danh sách restrictions
		for r := range restrictionMap {
			if strings.Contains(upper, r) || (len(upper) >= 3 && strings.Contains(r, upper)) {
				return fmt.Errorf("%w: nguyên liệu '%s' nằm trong danh sách dị ứng/kiêng cữ '%s'", domainerror.ErrAllergenConflict, trimmed, r)
			}
		}

		// 2. Tra cứu trong Catalog DB để kiểm tra AllergenTags nếu nguyên liệu đã tồn tại trong hệ thống
		for i := range activeCatalog {
			item := &activeCatalog[i]
			itemUpper := strings.ToUpper(item.Name())
			if itemUpper == upper || strings.Contains(itemUpper, upper) || strings.Contains(upper, itemUpper) {
				for _, tag := range item.AllergenTags() {
					tagUpper := strings.ToUpper(strings.TrimSpace(tag))
					if restrictionMap[tagUpper] {
						return fmt.Errorf("%w: nguyên liệu '%s' chứa chất dị ứng '%s' nằm trong danh sách kiêng cữ", domainerror.ErrAllergenConflict, trimmed, tagUpper)
					}
				}
			}
		}
	}

	return nil
}
