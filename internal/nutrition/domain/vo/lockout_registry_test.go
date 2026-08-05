package vo_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestLockoutRegistry_BR_NU_02_Rules(t *testing.T) {
	t.Parallel()

	now := time.Now()
	lockouts := []vo.LockoutItem{
		vo.NewLockoutItem(vo.LockoutTypeProtein, "CHICKEN", now.Add(vo.DurationProtein)),
		vo.NewLockoutItem(vo.LockoutTypeCarb, "SWEET_POTATO", now.Add(vo.DurationCarb)),
	}

	registry := vo.NewLockoutRegistry(lockouts)

	catalog := []vo.FoodNutrient{
		vo.NewFoodNutrient("1", "Ức gà tươi", "PROTEIN", 165.0, 31.0, 0.0, 3.6, nil, "CHICKEN", "", false),
		vo.NewFoodNutrient("2", "Cá hồi tươi", "PROTEIN", 208.0, 20.0, 0.0, 13.0, nil, "SALMON", "", false),
		vo.NewFoodNutrient("3", "Khoai lang luộc", "CARB", 90.0, 2.0, 21.0, 0.1, nil, "", "SWEET_POTATO", false),
		vo.NewFoodNutrient("4", "Gạo lứt luộc", "CARB", 112.0, 2.6, 23.0, 0.9, nil, "", "RICE", false),
	}

	available := registry.FilterAvailableIngredients(catalog, now)

	if len(available) != 2 {
		t.Fatalf("got %d available ingredients, want 2", len(available))
	}

	names := []string{available[0].Name(), available[1].Name()}
	if names[0] != "Cá hồi tươi" || names[1] != "Gạo lứt luộc" {
		t.Fatalf("unexpected available ingredients: %v", names)
	}
}

func TestLockoutRegistry_BR_NU_02_1_FilterAvailableIngredients(t *testing.T) {
	t.Parallel()

	now := time.Now()
	lockouts := []vo.LockoutItem{
		vo.NewLockoutItem(vo.LockoutTypeProtein, "BEEF", now.Add(vo.DurationProtein)),
	}

	registry := vo.NewLockoutRegistry(lockouts)

	candidates := []string{"BEEF", "CHICKEN", "SALMON"}
	available, locked := registry.CheckCollisions(candidates, now)

	if len(available) != 2 || len(locked) != 1 {
		t.Fatalf("got available %d, locked %d; want available 2, locked 1", len(available), len(locked))
	}

	if locked[0] != "BEEF" {
		t.Fatalf("got locked item %s, want BEEF", locked[0])
	}
}
