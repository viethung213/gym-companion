package vo_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestHistoricalConsumption(t *testing.T) {
	t.Parallel()

	now := time.Now()
	hc := vo.NewHistoricalConsumption("Ức gà áp chảo", []string{"Ức gà tươi", "Bông cải xanh"}, now)

	if hc.MealName != "Ức gà áp chảo" {
		t.Fatalf("got MealName %s, want Ức gà áp chảo", hc.MealName)
	}
	if len(hc.Ingredients) != 2 || hc.Ingredients[0] != "Ức gà tươi" {
		t.Fatalf("got Ingredients %v, want [Ức gà tươi, Bông cải xanh]", hc.Ingredients)
	}
	if !hc.ConsumedAt.Equal(now) {
		t.Fatalf("got ConsumedAt %v, want %v", hc.ConsumedAt, now)
	}
}
