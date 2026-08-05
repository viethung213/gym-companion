package vo

import "strings"

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
}

func NewFoodNutrient(
	id, name, category string,
	caloriesPer100g, proteinPer100g, carbsPer100g, fatPer100g float64,
	allergenTags []string,
	proteinSource, carbSource string,
	isNutiFoodProduct bool,
) FoodNutrient {
	tagsCopy := make([]string, len(allergenTags))
	copy(tagsCopy, allergenTags)

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
	}
}

func (f FoodNutrient) ID() string                { return f.id }
func (f FoodNutrient) Name() string              { return f.name }
func (f FoodNutrient) Category() string          { return f.category }
func (f FoodNutrient) CaloriesPer100g() float64   { return f.caloriesPer100g }
func (f FoodNutrient) ProteinPer100g() float64    { return f.proteinPer100g }
func (f FoodNutrient) CarbsPer100g() float64      { return f.carbsPer100g }
func (f FoodNutrient) FatPer100g() float64        { return f.fatPer100g }
func (f FoodNutrient) AllergenTags() []string     { return f.allergenTags }
func (f FoodNutrient) ProteinSource() string     { return f.proteinSource }
func (f FoodNutrient) CarbSource() string        { return f.carbSource }
func (f FoodNutrient) IsNutiFoodProduct() bool  { return f.isNutiFoodProduct }

func (f FoodNutrient) CalculateGramsForCalories(targetCalories float64) float64 {
	if f.caloriesPer100g <= 0 {
		return 100.0
	}
	return (targetCalories / f.caloriesPer100g) * 100.0
}

func (f FoodNutrient) MatchesSourceOrCategory(filter string) bool {
	filterUpper := strings.ToUpper(strings.TrimSpace(filter))
	return strings.ToUpper(f.category) == filterUpper ||
		strings.ToUpper(f.proteinSource) == filterUpper ||
		strings.ToUpper(f.carbSource) == filterUpper
}
