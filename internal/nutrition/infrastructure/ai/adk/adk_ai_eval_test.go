package adk_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	nutritionAdk "github.com/viethung213/gym-companion/internal/nutrition/infrastructure/ai/adk"
)

type mockFoodRepoForAIEval struct{}

func (m *mockFoodRepoForAIEval) FindByID(_ context.Context, id string) (*aggregate.FoodItem, error) {
	return aggregate.NewFoodItem(id, "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false), nil
}
func (m *mockFoodRepoForAIEval) FindByName(_ context.Context, name string) (*aggregate.FoodItem, error) {
	return nil, nil
}
func (m *mockFoodRepoForAIEval) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return []vo.FoodNutrient{
		vo.NewFoodNutrient("food-p1", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false),
		vo.NewFoodNutrient("food-c1", "Khoai lang luộc", "CARB", 90, 2, 21, 0.1, nil, "", "SWEET_POTATO", false),
		vo.NewFoodNutrient("food-v1", "Bông cải xanh", "VEGGIE", 34, 2.8, 7, 0.4, nil, "", "", false),
	}, nil
}
func (m *mockFoodRepoForAIEval) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *mockFoodRepoForAIEval) Save(_ context.Context, _ *aggregate.FoodItem) error   { return nil }
func (m *mockFoodRepoForAIEval) Update(_ context.Context, _ *aggregate.FoodItem) error { return nil }

// TestADKAI_PromptAndWorkflowIntegrity kiểm tra việc đọc và gán đúng các prompt file cho 4 luồng.
func TestADKAI_PromptAndWorkflowIntegrity(t *testing.T) {
	t.Parallel()

	promptFiles := []string{
		"prompts/generator.txt",
		"prompts/daily.txt",
		"prompts/post_workout.txt",
		"prompts/pantry.txt",
		"prompts/adhoc.txt",
	}

	for _, file := range promptFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read prompt file %s: %v", file, err)
		}
		if len(content) == 0 {
			t.Fatalf("prompt file %s is empty", file)
		}
	}
}

// TestADKAI_MultiIterationEvaluation thực hiện test lặp lại nhiều lần (N iterations)
// để kiểm tra chất lượng phản hồi, cấu trúc JSON contract, và tính ổn định của AI Agent.
func TestADKAI_MultiIterationEvaluation(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	foodRepo := &mockFoodRepoForAIEval{}

	agent, err := nutritionAdk.NewADKNutritionAgent(ctx, apiKey, foodRepo)
	if err != nil {
		t.Skipf("Skipping live AI Agent eval test (ADK init error or missing key): %v", err)
		return
	}

	catalog, _ := foodRepo.FindActiveCatalog(ctx)
	lockoutRegistry := vo.NewLockoutRegistry(nil)

	workflowsToTest := []struct {
		name     string
		mealType string
	}{
		{"Luồng 5h Sáng (Daily 4 Meals)", nutritionAdk.FlowDaily},
		{"Luồng Bù Calo Sau Tập (Post Workout)", nutritionAdk.FlowPostWorkout},
		{"Luồng Chế Biến Tủ Lạnh (Pantry)", nutritionAdk.FlowPantry},
		{"Luồng Gợi Ý Nhanh (Adhoc)", nutritionAdk.FlowAdhoc},
	}

	const iterations = 3 // Số lần chạy lặp lại để đánh giá độ ổn định

	for _, wf := range workflowsToTest {
		wf := wf
		t.Run(wf.name, func(t *testing.T) {
			successCount := 0

			for i := 1; i <= iterations; i++ {
				promptCtx := repository.AIMenuPromptContext{
					UserID:               "eval-user-123",
					PlanDate:             time.Now(),
					MealType:             wf.mealType,
					TargetMealCalories:   600.0,
					AvailableIngredients: catalog,
					UserRestrictions:     nil,
				}

				results, selectErr := agent.SelectCreativeMealOptions(ctx, promptCtx, lockoutRegistry)
				if selectErr != nil {
					t.Logf("Iteration %d/%d [%s] failed: %v", i, iterations, wf.name, selectErr)
					continue
				}

				if len(results) == 0 {
					t.Logf("Iteration %d/%d [%s] returned 0 meal options", i, iterations, wf.name)
					continue
				}

				// Verify JSON contract & options output
				for optIdx, opt := range results {
					if opt.RecipeName == "" {
						t.Errorf("Iteration %d Option %d has empty RecipeName", i, optIdx+1)
					}
					if len(opt.CookingSteps) == 0 {
						t.Errorf("Iteration %d Option %d has empty CookingSteps", i, optIdx+1)
					}
				}

				successCount++
			}

			t.Logf("Workflow [%s] Evaluation Pass Rate: %d/%d", wf.name, successCount, iterations)
		})
	}
}

// TestADKAI_JSONSchemaContractVerification kiểm tra tính hợp lệ của JSON Schema Output Contract.
func TestADKAI_JSONSchemaContractVerification(t *testing.T) {
	t.Parallel()

	sampleJSON := `{
		"options": [
			{
				"protein_food_id": "food-p1",
				"protein_food_name": "Ức gà tươi",
				"carb_food_id": "food-c1",
				"carb_food_name": "Khoai lang luộc",
				"veggie_food_id": "food-v1",
				"veggie_food_name": "Bông cải xanh",
				"cooking_style": "Áp chảo thảo mộc",
				"recipe_name": "Ức Gà Áp Chảo Thảo Mộc Với Khoai Lang",
				"cooking_steps": [
					"Sơ chế ức gà và khoai lang...",
					"Áp chảo ức gà với thảo mộc trong 15 phút...",
					"Dùng nóng."
				],
				"supplementary_ingredients": [
					{
						"name": "Dầu oliu",
						"amount_gram": 10.0,
						"is_nutifood_product": false
					}
				]
			}
		],
		"new_food_catalog_items": [
			{
				"name": "Sốt chanh dây tự làm",
				"category": "SAUCE",
				"calories_per_100g": 120.0,
				"protein_per_100g": 0.5,
				"carbs_per_100g": 25.0,
				"fat_per_100g": 1.0,
				"allergen_tags": []
			}
		]
	}`

	var plan nutritionAdk.GeneratedMealPlan
	err := json.Unmarshal([]byte(sampleJSON), &plan)
	if err != nil {
		t.Fatalf("failed to unmarshal sample AI contract JSON: %v", err)
	}

	if len(plan.Options) != 1 {
		t.Fatalf("got options len %d, want 1", len(plan.Options))
	}
	if plan.Options[0].RecipeName != "Ức Gà Áp Chảo Thảo Mộc Với Khoai Lang" {
		t.Fatalf("got recipe name %q, want %q", plan.Options[0].RecipeName, "Ức Gà Áp Chảo Thảo Mộc Với Khoai Lang")
	}
	if len(plan.NewFoodCatalogItems) != 1 {
		t.Fatalf("got new food catalog items len %d, want 1", len(plan.NewFoodCatalogItems))
	}
}
