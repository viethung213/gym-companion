package adk

import (
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type FoodNutrientDTO struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	CaloriesPer100g   float64  `json:"calories_per_100g"`
	ProteinPer100g    float64  `json:"protein_per_100g"`
	CarbsPer100g      float64  `json:"carbs_per_100g"`
	FatPer100g        float64  `json:"fat_per_100g"`
	AllergenTags      []string `json:"allergen_tags,omitempty"`
	ProteinSource     string   `json:"protein_source,omitempty"`
	CarbSource        string   `json:"carb_source,omitempty"`
	IsNutiFoodProduct bool     `json:"is_nutifood_product,omitempty"`
}

//nolint:gocritic // hugeParam: fn value object mapping
func ToFoodNutrientDTO(fn vo.FoodNutrient) FoodNutrientDTO {
	return FoodNutrientDTO{
		ID:                fn.ID(),
		Name:              fn.Name(),
		Category:          fn.Category(),
		CaloriesPer100g:   fn.CaloriesPer100g(),
		ProteinPer100g:    fn.ProteinPer100g(),
		CarbsPer100g:      fn.CarbsPer100g(),
		FatPer100g:        fn.FatPer100g(),
		AllergenTags:      fn.AllergenTags(),
		ProteinSource:     fn.ProteinSource(),
		CarbSource:        fn.CarbSource(),
		IsNutiFoodProduct: fn.IsNutiFoodProduct(),
	}
}

func ToFoodNutrientDTOs(fns []vo.FoodNutrient) []FoodNutrientDTO {
	dtos := make([]FoodNutrientDTO, 0, len(fns))
	for i := range fns {
		dtos = append(dtos, ToFoodNutrientDTO(fns[i]))
	}
	return dtos
}

//nolint:gocritic // hugeParam: dto struct value object mapping
func (dto FoodNutrientDTO) ToDomain() vo.FoodNutrient {
	return vo.NewFoodNutrient(
		dto.ID,
		dto.Name,
		dto.Category,
		dto.CaloriesPer100g,
		dto.ProteinPer100g,
		dto.CarbsPer100g,
		dto.FatPer100g,
		dto.AllergenTags,
		dto.ProteinSource,
		dto.CarbSource,
		dto.IsNutiFoodProduct,
	)
}

func ToFoodNutrientDomains(dtos []FoodNutrientDTO) []vo.FoodNutrient {
	domains := make([]vo.FoodNutrient, 0, len(dtos))
	for i := range dtos {
		domains = append(domains, dtos[i].ToDomain())
	}
	return domains
}

type NutritionPromptContext struct {
	UserID                   string             `json:"user_id"`
	PlanDate                 time.Time          `json:"plan_date"`
	MealType                 string             `json:"meal_type"`
	NutritionGoal            string             `json:"nutrition_goal,omitempty"`
	TargetDailyCalories      float64            `json:"target_daily_calories,omitempty"`
	TargetMealCalories       float64            `json:"target_meal_calories"`
	TargetProteinGrams       float64            `json:"target_protein_grams,omitempty"`
	TargetCarbGrams          float64            `json:"target_carb_grams,omitempty"`
	TargetFatGrams           float64            `json:"target_fat_grams,omitempty"`
	UserRestrictions         []string           `json:"user_restrictions"`
	MandatoryUserIngredients []FoodNutrientDTO  `json:"mandatory_user_ingredients,omitempty"`
	SupplementaryIngredients []FoodNutrientDTO  `json:"supplementary_db_candidates,omitempty"`
	AvailableIngredients     []FoodNutrientDTO  `json:"available_ingredients"`
	LockoutRegistry          vo.LockoutRegistry `json:"-"`
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
	NewFoodCatalogItems []NewFoodItemSpec     `json:"new_food_catalog_items,omitempty"`
}

// CompactMealOptionForReview là DTO tinh gọn cho Reviewer Agent,
// loại bỏ các UUID (protein_food_id, carb_food_id, veggie_food_id) và trường dư thừa
// nhằm giảm token input đáng kể cho reviewer mà vẫn giữ đủ thông tin ẩm thực và dinh dưỡng.
type CompactMealOptionForReview struct {
	OptionIndex              int                           `json:"option_index"`
	RecipeName               string                        `json:"recipe_name"`
	CookingStyle             string                        `json:"cooking_style"`
	ProteinFoodName          string                        `json:"protein_food_name"`
	CarbFoodName             string                        `json:"carb_food_name"`
	VeggieFoodName           string                        `json:"veggie_food_name"`
	CookingSteps             []string                      `json:"cooking_steps"`
	SupplementaryIngredients []SupplementaryIngredientSpec `json:"supplementary_ingredients,omitempty"`
	TotalProteinGrams        float64                       `json:"total_protein_grams"`
	TotalCarbGrams           float64                       `json:"total_carb_grams"`
	TotalFatGrams            float64                       `json:"total_fat_grams"`
}

// CompactMealPlanForReview là danh sách các món ăn đã được nén cho Reviewer.
type CompactMealPlanForReview struct {
	Options []CompactMealOptionForReview `json:"options"`
}

// ToCompactForReview chuyển đổi GeneratedMealPlan sang định dạng nén cho Reviewer Agent.
func (p *GeneratedMealPlan) ToCompactForReview() *CompactMealPlanForReview {
	if p == nil {
		return nil
	}
	compactOptions := make([]CompactMealOptionForReview, 0, len(p.Options))
	for i := range p.Options {
		opt := &p.Options[i]
		compactOptions = append(compactOptions, CompactMealOptionForReview{
			OptionIndex:              i + 1,
			RecipeName:               opt.RecipeName,
			CookingStyle:             opt.CookingStyle,
			ProteinFoodName:          opt.ProteinFoodName,
			CarbFoodName:             opt.CarbFoodName,
			VeggieFoodName:           opt.VeggieFoodName,
			CookingSteps:             opt.CookingSteps,
			SupplementaryIngredients: opt.SupplementaryIngredients,
			TotalProteinGrams:        opt.TotalProteinGrams,
			TotalCarbGrams:           opt.TotalCarbGrams,
			TotalFatGrams:            opt.TotalFatGrams,
		})
	}
	return &CompactMealPlanForReview{
		Options: compactOptions,
	}
}

type ValidationOutcome struct {
	Plan   *GeneratedMealPlan
	Issues []string
}

type ValidationReport struct {
	Passed   bool     `json:"passed"`
	Degraded bool     `json:"degraded"`
	Issues   []string `json:"issues"`
}

type ReviewNote struct {
	OptionIndex int    `json:"option_index,omitempty"`
	RecipeName  string `json:"recipe_name,omitempty"`
	Area        string `json:"area"`
	Detail      string `json:"detail"`
	Fix         string `json:"fix"`
}

type PreviousFeedbackItem struct {
	Area     string `json:"area"`
	Applied  bool   `json:"applied"`
	Evidence string `json:"evidence"`
}

type MealPlanReview struct {
	Approved         bool                   `json:"approved"`
	Score            int                    `json:"score"`
	Confidence       float64                `json:"confidence"`
	Feedback         []ReviewNote           `json:"feedback"`
	Notes            []string               `json:"notes"`
	PreviousFeedback []PreviousFeedbackItem `json:"previous_feedback"`
}

func (r *MealPlanReview) unappliedAreas() []string {
	if r == nil {
		return nil
	}
	var unapplied []string
	for _, item := range r.PreviousFeedback {
		if !item.Applied {
			unapplied = append(unapplied, item.Area)
		}
	}
	return unapplied
}

type ReviewRequest struct {
	OriginalTask        string                    `json:"original_task"`
	NutritionGoal       string                    `json:"nutrition_goal,omitempty"`
	TargetDailyCalories float64                   `json:"target_daily_calories,omitempty"`
	TargetProteinGrams  float64                   `json:"target_protein_grams,omitempty"`
	TargetCarbGrams     float64                   `json:"target_carb_grams,omitempty"`
	TargetFatGrams      float64                   `json:"target_fat_grams,omitempty"`
	UserRestrictions    []string                  `json:"user_restrictions"`
	GeneratorOutput     *CompactMealPlanForReview `json:"generator_output"`
	ValidationResult    ValidationReport          `json:"validation_result"`
	ReviewRound         int                       `json:"review_round"`
	PreviousFeedback    []ReviewNote              `json:"previous_feedback"`
}

type ReviewRoundLog struct {
	Round       int          `json:"round"`
	Approved    bool         `json:"approved"`
	Score       int          `json:"score"`
	Confidence  float64      `json:"confidence"`
	Feedback    []ReviewNote `json:"feedback,omitempty"`
	Notes       []string     `json:"notes,omitempty"`
	RepairedOpt []string     `json:"repaired_options,omitempty"`
}

type PlanResult struct {
	Plan       *GeneratedMealPlan
	Review     *MealPlanReview
	Degraded   bool
	Issues     []string
	Attempts   int
	ReviewLogs []ReviewRoundLog
}
