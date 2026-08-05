package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestMealHistory_AddMealLog(t *testing.T) {
	t.Parallel()

	registry := vo.NewLockoutRegistry(nil)
	history := aggregate.NewMealHistory("hist-1", "user-1", registry)

	now := time.Now()
	logItem := aggregate.NewMealLog("log-1", "hist-1", "user-1", "Lunch", "Ức gà luộc", "1 dĩa", 400, 35, 10, 5, now)

	history.AddMealLog(logItem)

	if got := len(history.MealLogs()); got != 1 {
		t.Fatalf("got meal logs len %d, want 1", got)
	}

	cal, p, c, f := history.CalculateConsumedToday(now)
	if cal != 400 || p != 35 || c != 10 || f != 5 {
		t.Fatalf("got consumed calories %f, p %f, c %f, f %f; want 400, 35, 10, 5", cal, p, c, f)
	}
}

func TestMealHistory_ApplyLockoutRule(t *testing.T) {
	t.Parallel()

	registry := vo.NewLockoutRegistry(nil)
	history := aggregate.NewMealHistory("hist-2", "user-2", registry)

	now := time.Now()
	history.ApplyLockoutRule("PROTEIN", "Ức gà tươi", 7*24*time.Hour, now)

	reg := history.LockoutRegistry()
	food := vo.NewFoodNutrient("f1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)

	filtered := reg.FilterAvailableIngredients([]vo.FoodNutrient{food}, now.Add(24*time.Hour))
	if len(filtered) != 0 {
		t.Fatalf("expected food to be locked out, but was available")
	}
}
