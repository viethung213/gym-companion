package adk

import (
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type NutritionPromptContext struct {
	UserID               string            `json:"user_id"`
	PlanDate             time.Time         `json:"plan_date"`
	MealType             string            `json:"meal_type"`
	TargetMealCalories   float64           `json:"target_meal_calories"`
	TargetProteinGrams   float64           `json:"target_protein_grams"`
	TargetCarbGrams      float64           `json:"target_carb_grams"`
	TargetFatGrams        float64           `json:"target_fat_grams"`
	UserRestrictions     []string          `json:"user_restrictions"`
	AvailableIngredients []vo.FoodNutrient `json:"available_ingredients"`
}

type SupplementaryIngredientSpec struct {
	Name              string  `json:"name"`
	AmountGram        float64 `json:"amount_gram"`
	IsNutiFoodProduct bool    `json:"is_nutifood_product"`
}

type NewFoodItemSpec struct {
	Name            string   `json:"name"`
	Category        string   `json:"category"`
	CaloriesPer100g float64  `json:"calories_per_100g"`
	ProteinPer100g  float64  `json:"protein_per_100g"`
	CarbsPer100g    float64  `json:"carbs_per_100g"`
	FatPer100g      float64  `json:"fat_per_100g"`
	AllergenTags    []string `json:"allergen_tags"`
}

// GeneratedMealOption là 1 option bữa ăn do AI sinh ra.
// Các field Total*Grams là tổng macro AI đã tính sẵn dựa theo gram nguyên liệu.
type GeneratedMealOption struct {
	ProteinFoodID            string                        `json:"protein_food_id"`
	ProteinFoodName          string                        `json:"protein_food_name"`
	CarbFoodID               string                        `json:"carb_food_id"`
	CarbFoodName             string                        `json:"carb_food_name"`
	VeggieFoodID             string                        `json:"veggie_food_id"`
	VeggieFoodName           string                        `json:"veggie_food_name"`
	CookingStyle             string                        `json:"cooking_style"`
	RecipeName               string                        `json:"recipe_name"`
	CookingSteps             []string                      `json:"cooking_steps"`
	SupplementaryIngredients []SupplementaryIngredientSpec `json:"supplementary_ingredients"`
	TotalProteinGrams        float64                       `json:"total_protein_grams"`
	TotalCarbGrams           float64                       `json:"total_carb_grams"`
	TotalFatGrams            float64                       `json:"total_fat_grams"`
}

type GeneratedMealPlan struct {
	Options             []GeneratedMealOption `json:"options"`
	NewFoodCatalogItems []NewFoodItemSpec     `json:"new_food_catalog_items"`
}

type ValidationOutcome struct {
	Plan   *GeneratedMealPlan
	Issues []string
}

type PlanResult struct {
	Plan     *GeneratedMealPlan
	Degraded bool
}
