package service

import (
	"testing"
)

func TestFilterAllergensOnly(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()

	// Empty restrictions returns whole catalog
	all := FilterAllergensOnly(catalog, nil)
	if len(all) != len(catalog) {
		t.Fatalf("expected full catalog (%d), got %d", len(catalog), len(all))
	}

	// Filter EGG and SHELLFISH
	filtered := FilterAllergensOnly(catalog, []string{"EGG", "SHELLFISH"})
	for _, f := range filtered {
		if f.Name() == "Trứng gà tươi" || f.Name() == "Tôm sú tươi" {
			t.Errorf("expected allergen item %s to be filtered out", f.Name())
		}
	}
}

func TestCategorizeFoods(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()
	proteins, carbs, veggies, snacks, _ := CategorizeFoods(catalog)

	if len(proteins) == 0 {
		t.Errorf("expected non-empty proteins")
	}
	if len(carbs) == 0 {
		t.Errorf("expected non-empty carbs")
	}
	if len(veggies) == 0 {
		t.Errorf("expected non-empty veggies")
	}
	if len(snacks) == 0 {
		t.Errorf("expected non-empty snacks")
	}

	for _, p := range proteins {
		if p.Category() != "PROTEIN" && p.ProteinPer100g() < 18 {
			t.Errorf("unexpected protein item: %s", p.Name())
		}
	}
}
