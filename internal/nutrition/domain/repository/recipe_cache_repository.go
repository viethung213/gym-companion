package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
)

type CachedRecipe struct {
	ID                       string
	IngredientHash           string
	RecipeName               string
	CookingStyle             string
	Ingredients              []aggregate.IngredientGram
	CookingSteps             []string
	SupplementaryIngredients []aggregate.IngredientGram
	CreatedAt                time.Time
}

type RecipeCacheRepository interface {
	FindByHash(ctx context.Context, hash string) (*CachedRecipe, error)
	Save(ctx context.Context, recipe *CachedRecipe) error
}
