package vo

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CostTierBudget   = "BUDGET"
	CostTierStandard = "STANDARD"
	CostTierPremium  = "PREMIUM"
)

var (
	ErrEmptyFoodName     = errors.New("food item name cannot be empty")
	ErrEmptyFoodCategory = errors.New("food item category cannot be empty")
	ErrNegativePrice     = errors.New("food item price per 100g cannot be negative")
	ErrInvalidCostTier   = errors.New("invalid food item cost tier: must be BUDGET, STANDARD, or PREMIUM")
)

// ValidateFoodItem kiểm tra đầy đủ tính hợp lệ của nguyên liệu thực phẩm:
// tên không rỗng, danh mục không rỗng, đơn giá không âm và phân khúc chi phí hợp lệ.
func ValidateFoodItem(name, category string, pricePer100g float64, costTier string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", ErrEmptyFoodName
	}
	if strings.TrimSpace(category) == "" {
		return "", ErrEmptyFoodCategory
	}
	if pricePer100g < 0 {
		return "", ErrNegativePrice
	}

	normalized := strings.ToUpper(strings.TrimSpace(costTier))
	if normalized == "" {
		normalized = CostTierStandard
	}

	switch normalized {
	case CostTierBudget, CostTierStandard, CostTierPremium:
		return normalized, nil
	default:
		return "", fmt.Errorf("%w: got %q", ErrInvalidCostTier, costTier)
	}
}

// NormalizeCostTier chuẩn hóa cost tier từ dữ liệu persistence, fallback về STANDARD nếu không hợp lệ.
func NormalizeCostTier(costTier string) string {
	tier := strings.ToUpper(strings.TrimSpace(costTier))
	switch tier {
	case CostTierBudget, CostTierStandard, CostTierPremium:
		return tier
	default:
		return CostTierStandard
	}
}

// FoodNutrient là Value Object đại diện cho một nguyên liệu thực phẩm đầy đủ
// gồm thông tin dinh dưỡng, phân loại nguồn gốc, dị ứng và định giá chi phí.
type FoodNutrient struct {
	id                string
	name              string
	category          string
	caloriesPer100g   float64
	proteinPer100g    float64
	carbsPer100g      float64
	fatPer100g        float64
	allergenTags      []string
	proteinSource     string
	carbSource        string
	isNutiFoodProduct bool
	pricePer100g      float64
	costTier          string
}

func NewFoodNutrient(
	id, name, category string,
	caloriesPer100g, proteinPer100g, carbsPer100g, fatPer100g float64,
	allergenTags []string,
	proteinSource, carbSource string,
	isNutiFoodProduct bool,
	pricePer100g float64,
	costTier string,
) FoodNutrient {
	tagsCopy := make([]string, len(allergenTags))
	copy(tagsCopy, allergenTags)

	if pricePer100g < 0 {
		pricePer100g = 0
	}

	return FoodNutrient{
		id:                id,
		name:              name,
		category:          category,
		caloriesPer100g:   caloriesPer100g,
		proteinPer100g:    proteinPer100g,
		carbsPer100g:      carbsPer100g,
		fatPer100g:        fatPer100g,
		allergenTags:      tagsCopy,
		proteinSource:     proteinSource,
		carbSource:        carbSource,
		isNutiFoodProduct: isNutiFoodProduct,
		pricePer100g:      pricePer100g,
		costTier:          NormalizeCostTier(costTier),
	}
}

func (f *FoodNutrient) ID() string               { return f.id }
func (f *FoodNutrient) Name() string             { return f.name }
func (f *FoodNutrient) Category() string         { return f.category }
func (f *FoodNutrient) CaloriesPer100g() float64 { return f.caloriesPer100g }
func (f *FoodNutrient) ProteinPer100g() float64  { return f.proteinPer100g }
func (f *FoodNutrient) CarbsPer100g() float64    { return f.carbsPer100g }
func (f *FoodNutrient) FatPer100g() float64      { return f.fatPer100g }
func (f *FoodNutrient) AllergenTags() []string   { return f.allergenTags }
func (f *FoodNutrient) ProteinSource() string    { return f.proteinSource }
func (f *FoodNutrient) CarbSource() string       { return f.carbSource }
func (f *FoodNutrient) IsNutiFoodProduct() bool  { return f.isNutiFoodProduct }
func (f *FoodNutrient) PricePer100g() float64    { return f.pricePer100g }
func (f *FoodNutrient) CostTier() string         { return f.costTier }

func (f *FoodNutrient) CalculateGramsForCalories(targetCalories float64) float64 {
	if f.caloriesPer100g <= 0 {
		return 100.0
	}
	return (targetCalories / f.caloriesPer100g) * 100.0
}

func (f *FoodNutrient) MatchesSourceOrCategory(filter string) bool {
	filter = strings.TrimSpace(filter)
	return strings.EqualFold(f.category, filter) ||
		strings.EqualFold(f.proteinSource, filter) ||
		strings.EqualFold(f.carbSource, filter)
}
