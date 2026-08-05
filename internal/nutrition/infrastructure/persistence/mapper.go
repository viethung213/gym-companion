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

	return &repository.CachedRecipe{
		ID:             g.ID,
		IngredientHash: g.IngredientHash,
		RecipeName:     g.RecipeName,
		CookingStyle:   g.CookingStyle,
		CookingSteps:   steps,
		CreatedAt:      g.CreatedAt,
	}
}
