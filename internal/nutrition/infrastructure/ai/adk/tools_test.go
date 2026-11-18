package adk

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockFoodRepoForTools struct{}

func (m *mockFoodRepoForTools) FindByID(_ context.Context, _ string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *mockFoodRepoForTools) FindByName(_ context.Context, _ string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *mockFoodRepoForTools) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false, 13000, "BUDGET"),
		vo.NewFoodNutrient("2", "Khoai lang", "CARB", 90, 2, 21, 0.1, nil, "", "", false, 4500, "BUDGET"),
	}, nil
}
func (m *mockFoodRepoForTools) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("3", "Sữa NutiFood Varna", "NUTIFOOD", 80, 4, 10, 2.5, nil, "", "", true, 28000, "PREMIUM"),
	}, nil
}
func (m *mockFoodRepoForTools) Save(_ context.Context, _ *aggregate.FoodItem) error   { return nil }
func (m *mockFoodRepoForTools) Update(_ context.Context, _ *aggregate.FoodItem) error { return nil }

func TestNutritionTools(t *testing.T) {
	t.Parallel()

	tools := NewNutritionTools(&mockFoodRepoForTools{})

	// 1. FetchActiveFoodCatalog
	catalog, err := tools.FetchActiveFoodCatalog(context.Background(), "", "PROTEIN")
	if err != nil {
		t.Fatalf("unexpected error fetching active catalog: %v", err)
	}
	if len(catalog) != 1 || catalog[0].Category != "PROTEIN" {
		t.Fatalf("got catalog len %d, want 1 protein item", len(catalog))
	}

	// 2. CalculateMacroGramRatio
	p, c, f := tools.CalculateMacroGramRatio(2000, 0.30, 0.40, 0.30)
	if p != 150 || c != 200 || f != (600.0/9.0) {
		t.Fatalf("got p %f, c %f, f %f; want 150, 200, 66.67", p, c, f)
	}

	// 3. SuggestNutiFoodSupplement
	nuti, err := tools.SuggestNutiFoodSupplement(context.Background())
	if err != nil {
		t.Fatalf("unexpected error suggesting nutifood: %v", err)
	}
	if len(nuti) != 1 {
		t.Fatalf("got nutifood products len %d, want 1", len(nuti))
	}

	// 5. FetchActiveFoodCatalog với Session Context (Allergen restriction filtering)
	sessionID := "sess-123"
	tools.RegisterSessionContext(sessionID, &SessionFilterContext{
		UserID:           "u1",
		UserRestrictions: []string{"Ức gà"},
		PlanDate:         time.Now(),
	})
	defer tools.UnregisterSessionContext(sessionID)

	restrictedCatalog, err := tools.FetchActiveFoodCatalog(context.Background(), sessionID, "PROTEIN")
	if err != nil {
		t.Fatalf("unexpected error fetching catalog with session: %v", err)
	}
	// "Ức gà" bị loại bởi dị ứng -> kết quả phải rỗng
	if len(restrictedCatalog) != 0 {
		t.Fatalf("expected restricted allergen item to be filtered out, got %d items", len(restrictedCatalog))
	}

	// 6. FetchActiveFoodCatalog với ExcludeIDs
	excludedCatalog, err := tools.FetchActiveFoodCatalog(context.Background(), "", "CARB", FetchCatalogOptions{
		ExcludeIDs: []string{"2"},
	})
	if err != nil {
		t.Fatalf("unexpected error fetching catalog with exclude: %v", err)
	}
	if len(excludedCatalog) != 0 {
		t.Fatalf("expected item '2' to be excluded, got %d items", len(excludedCatalog))
	}

	// 7. FetchActiveFoodCatalog với Page
	pagedCatalog, err := tools.FetchActiveFoodCatalog(context.Background(), "", "CARB", FetchCatalogOptions{
		Page: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error fetching catalog with page: %v", err)
	}
	if len(pagedCatalog) != 1 || pagedCatalog[0].ID != "2" {
		t.Fatalf("expected 1 item for page 1, got %v", pagedCatalog)
	}
}
