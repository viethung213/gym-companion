package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type FoodItemRepository interface {
	FindByID(ctx context.Context, id string) (*aggregate.FoodItem, error)
	FindByName(ctx context.Context, name string) (*aggregate.FoodItem, error)
	FindActiveCatalog(ctx context.Context) ([]vo.FoodNutrient, error)
	FindNutiFoodProducts(ctx context.Context) ([]vo.FoodNutrient, error)
	Save(ctx context.Context, item *aggregate.FoodItem) error
	Update(ctx context.Context, item *aggregate.FoodItem) error
}
