package persistence

import (
	"encoding/json"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func (g *GormFoodItem) ToDomain() *aggregate.FoodItem {
	var tags []string
	if len(g.AllergenTagsJSON) > 0 {
		_ = json.Unmarshal(g.AllergenTagsJSON, &tags)
	}

	return aggregate.ReconstructFoodItem(
		g.ID,
		g.Name,
		g.Category,
		g.CaloriesPer100g,
		g.ProteinPer100g,
		g.CarbsPer100g,
		g.FatPer100g,
		tags,
		g.ProteinSource,
		g.CarbSource,
		g.IsNutiFoodProduct,
		g.Status,
		g.CreatedAt,
		g.UpdatedAt,
	)
}

func (g *GormFoodItem) ToNutrientDomain() vo.FoodNutrient {
	var tags []string
	if len(g.AllergenTagsJSON) > 0 {
		_ = json.Unmarshal(g.AllergenTagsJSON, &tags)
	}

	return vo.NewFoodNutrient(
		g.ID,
		g.Name,
		g.Category,
		g.CaloriesPer100g,
		g.ProteinPer100g,
		g.CarbsPer100g,
		g.FatPer100g,
		tags,
		g.ProteinSource,
		g.CarbSource,
		g.IsNutiFoodProduct,
	)
}

func FromDomainFoodItem(item *aggregate.FoodItem) *GormFoodItem {
	tagsJSON, _ := json.Marshal(item.AllergenTags())
	return &GormFoodItem{
		ID:                item.ID(),
		Name:              item.Name(),
		Category:          item.Category(),
		CaloriesPer100g:   item.CaloriesPer100g(),
		ProteinPer100g:    item.ProteinPer100g(),
		CarbsPer100g:      item.CarbsPer100g(),
		FatPer100g:        item.FatPer100g(),
		AllergenTagsJSON:  tagsJSON,
		ProteinSource:     item.ProteinSource(),
		CarbSource:        item.CarbSource(),
		IsNutiFoodProduct: item.IsNutiFoodProduct(),
		Status:            item.Status(),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

func (g *GormRecipe) ToDomain() *repository.CachedRecipe {
	var steps []string
	if len(g.CookingStepsJSON) > 0 {
		_ = json.Unmarshal(g.CookingStepsJSON, &steps)
	}

	ings, _ := UnmarshalIngredientsJSON(g.IngredientsJSON)

	return &repository.CachedRecipe{
		ID:             g.ID,
		IngredientHash: g.IngredientHash,
		RecipeName:     g.RecipeName,
		CookingStyle:   g.CookingStyle,
		Ingredients:    ings,
		CookingSteps:   steps,
		CreatedAt:      g.CreatedAt,
	}
}

type jsonIngredientGramDTO struct {
	IngredientName  string  `json:"ingredientName"`
	Grams           float64 `json:"grams"`
	IsSupplementary bool    `json:"isSupplementary"`
}

type jsonMealOptionDTO struct {
	OptionID          string                  `json:"optionId"`
	MealName          string                  `json:"mealName"`
	Calories          float64                 `json:"calories"`
	ProteinGrams      float64                 `json:"proteinGrams"`
	CarbGrams         float64                 `json:"carbGrams"`
	FatGrams          float64                 `json:"fatGrams"`
	Ingredients       []jsonIngredientGramDTO `json:"ingredients"`
	CookingSteps      []string                `json:"cookingSteps"`
	IsLogged          bool                    `json:"isLogged"`
	IsNutiFoodProduct bool                    `json:"isNutiFoodProduct"`
}

type jsonDailyMealDTO struct {
	MealType      string              `json:"mealType"`
	Options       []jsonMealOptionDTO `json:"options"`
	ScheduledTime string              `json:"scheduledTime"`
}

func MarshalDailyMealsJSON(meals []aggregate.DailyMeal) ([]byte, error) {
	dtos := make([]jsonDailyMealDTO, 0, len(meals))
	for _, m := range meals {
		optDTOs := make([]jsonMealOptionDTO, 0, len(m.Options()))
		for _, opt := range m.Options() {
			ingDTOs := make([]jsonIngredientGramDTO, 0, len(opt.Ingredients()))
			for _, ing := range opt.Ingredients() {
				ingDTOs = append(ingDTOs, jsonIngredientGramDTO{
					IngredientName:  ing.IngredientName(),
					Grams:           ing.Grams(),
					IsSupplementary: ing.IsSupplementary(),
				})
			}
			optDTOs = append(optDTOs, jsonMealOptionDTO{
				OptionID:          opt.OptionID(),
				MealName:          opt.MealName(),
				Calories:          opt.Calories(),
				ProteinGrams:      opt.ProteinGrams(),
				CarbGrams:         opt.CarbGrams(),
				FatGrams:          opt.FatGrams(),
				Ingredients:       ingDTOs,
				CookingSteps:      opt.CookingSteps(),
				IsLogged:          opt.IsLogged(),
				IsNutiFoodProduct: opt.IsNutiFoodProduct(),
			})
		}
		dtos = append(dtos, jsonDailyMealDTO{
			MealType:      m.MealType(),
			Options:       optDTOs,
			ScheduledTime: m.ScheduledTime(),
		})
	}
	return json.Marshal(dtos)
}

func UnmarshalDailyMealsJSON(data []byte) ([]aggregate.DailyMeal, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var dtos []jsonDailyMealDTO
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, err
	}
	meals := make([]aggregate.DailyMeal, 0, len(dtos))
	for _, dto := range dtos {
		opts := make([]aggregate.MealOption, 0, len(dto.Options))
		for _, optDTO := range dto.Options {
			ings := make([]aggregate.IngredientGram, 0, len(optDTO.Ingredients))
			for _, ingDTO := range optDTO.Ingredients {
				ings = append(ings, aggregate.NewIngredientGram(ingDTO.IngredientName, ingDTO.Grams, ingDTO.IsSupplementary))
			}
			opts = append(opts, aggregate.ReconstructMealOption(
				optDTO.OptionID,
				optDTO.MealName,
				optDTO.Calories,
				optDTO.ProteinGrams,
				optDTO.CarbGrams,
				optDTO.FatGrams,
				ings,
				optDTO.CookingSteps,
				optDTO.IsLogged,
				optDTO.IsNutiFoodProduct,
			))
		}
		meals = append(meals, aggregate.NewDailyMealWithSchedule(dto.MealType, opts, dto.ScheduledTime))
	}
	return meals, nil
}

func MarshalIngredientsJSON(ingredients []aggregate.IngredientGram) ([]byte, error) {
	dtos := make([]jsonIngredientGramDTO, 0, len(ingredients))
	for _, ing := range ingredients {
		dtos = append(dtos, jsonIngredientGramDTO{
			IngredientName:  ing.IngredientName(),
			Grams:           ing.Grams(),
			IsSupplementary: ing.IsSupplementary(),
		})
	}
	return json.Marshal(dtos)
}

func UnmarshalIngredientsJSON(data []byte) ([]aggregate.IngredientGram, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var dtos []jsonIngredientGramDTO
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, err
	}
	ings := make([]aggregate.IngredientGram, 0, len(dtos))
	for _, dto := range dtos {
		ings = append(ings, aggregate.NewIngredientGram(dto.IngredientName, dto.Grams, dto.IsSupplementary))
	}
	return ings, nil
}
