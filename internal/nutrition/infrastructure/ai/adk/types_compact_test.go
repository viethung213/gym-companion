package adk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToCompactForReview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan *GeneratedMealPlan
		want int
	}{
		{
			name: "nil plan returns nil",
			plan: nil,
			want: 0,
		},
		{
			name: "compact plan preserves culinary and nutrition fields while stripping IDs",
			plan: &GeneratedMealPlan{
				Options: []GeneratedMealOption{
					{
						ProteinFoodID:   "uuid-protein-1",
						ProteinFoodName: "Ức gà",
						CarbFoodID:      "uuid-carb-1",
						CarbFoodName:    "Gạo lứt",
						VeggieFoodID:    "uuid-veggie-1",
						VeggieFoodName:  "Bông cải xanh",
						CookingStyle:    "Áp chảo",
						RecipeName:      "Cơm ức gà áp chảo",
						CookingSteps: []string{
							"Ướp ức gà",
							"Áp chảo chín đều",
						},
						SupplementaryIngredients: []SupplementaryIngredientSpec{
							{
								Name:       "Dầu olive",
								AmountGram: 10,
							},
						},
						TotalProteinGrams: 35,
						TotalCarbGrams:    45,
						TotalFatGrams:     10,
					},
				},
				NewFoodCatalogItems: []NewFoodItemSpec{
					{Name: "Novel Food"},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.plan.ToCompactForReview()
			if tt.plan == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}

			if len(got.Options) != tt.want {
				t.Fatalf("got %d options, want %d", len(got.Options), tt.want)
			}

			opt := got.Options[0]
			if opt.RecipeName != "Cơm ức gà áp chảo" {
				t.Errorf("got RecipeName %s, want %s", opt.RecipeName, "Cơm ức gà áp chảo")
			}
			if opt.ProteinFoodName != "Ức gà" || opt.CarbFoodName != "Gạo lứt" || opt.VeggieFoodName != "Bông cải xanh" {
				t.Errorf("food names mismatch: %+v", opt)
			}
			if opt.TotalProteinGrams != 35 || opt.TotalCarbGrams != 45 || opt.TotalFatGrams != 10 {
				t.Errorf("macro mismatch: %+v", opt)
			}

			// Verify JSON marshaling: does not contain any uuid or food_id keys
			req := ReviewRequest{
				OriginalTask:    "DAILY_4_MEALS",
				GeneratorOutput: got,
			}
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("failed to marshal ReviewRequest: %v", err)
			}

			jsonStr := string(data)
			if strings.Contains(jsonStr, "uuid-protein-1") || strings.Contains(jsonStr, "protein_food_id") {
				t.Errorf("compact review request must not contain food IDs: %s", jsonStr)
			}
			if strings.Contains(jsonStr, "carb_food_id") || strings.Contains(jsonStr, "veggie_food_id") {
				t.Errorf("compact review request must not contain carb/veggie IDs: %s", jsonStr)
			}
			if strings.Contains(jsonStr, "new_food_catalog_items") {
				t.Errorf("compact review request must not contain new_food_catalog_items: %s", jsonStr)
			}
		})
	}
}
