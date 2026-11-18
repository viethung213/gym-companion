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

func TestValidatePantryIngredientsAgainstAllergens(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()

	t.Run("safe ingredients pass without error", func(t *testing.T) {
		err := ValidatePantryIngredientsAgainstAllergens(
			[]string{"Ức gà", "Bông cải xanh", "Thịt nai"},
			catalog,
			[]string{"SHELLFISH", "EGG"},
		)
		if err != nil {
			t.Errorf("expected no error for safe ingredients, got %v", err)
		}
	})

	t.Run("conflict with allergen tag in catalog", func(t *testing.T) {
		err := ValidatePantryIngredientsAgainstAllergens(
			[]string{"Tôm sú tươi", "Cơm trắng"},
			catalog,
			[]string{"SHELLFISH"},
		)
		if err == nil {
			t.Errorf("expected error when user sends shellfish, got nil")
		}
	})

	t.Run("conflict directly by name matching", func(t *testing.T) {
		err := ValidatePantryIngredientsAgainstAllergens(
			[]string{"Đậu phộng rang", "Thịt bò"},
			catalog,
			[]string{"ĐẬU PHỘNG"},
		)
		if err == nil {
			t.Errorf("expected error when user sends ingredient directly matching restriction, got nil")
		}
	})
}

func TestInferIngredientCategory(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		expected string
	}{
		{"Thịt nai", "PROTEIN"},
		{"Ức gà tươi phi lê", "PROTEIN"},
		{"Cá hồi Na Uy", "PROTEIN"},
		{"Tôm càng xanh", "PROTEIN"},
		{"Venison steak", "PROTEIN"},
		{"Cơm gạo lứt", "CARB"},
		{"Khoai lang mật", "CARB"},
		{"Brown rice", "CARB"},
		{"Bông cải xanh", "VEGGIE"},
		{"Rau muống non", "VEGGIE"},
		{"Fresh asparagus", "VEGGIE"},
		{"Dầu mè", "OTHER"},
	}

	for _, tc := range testCases {
		res := InferIngredientCategory(tc.name)
		if res != tc.expected {
			t.Errorf("InferIngredientCategory(%q) = %q; want %q", tc.name, res, tc.expected)
		}
	}
}
