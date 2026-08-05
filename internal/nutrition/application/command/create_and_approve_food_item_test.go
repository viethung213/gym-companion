package command_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockFoodRepoFull struct {
	items map[string]*aggregate.FoodItem
}

func (m *mockFoodRepoFull) FindByID(_ context.Context, id string) (*aggregate.FoodItem, error) {
	return m.items[id], nil
}

func (m *mockFoodRepoFull) FindByName(_ context.Context, name string) (*aggregate.FoodItem, error) {
	for _, item := range m.items {
		if item.Name() == name {
			return item, nil
		}
	}
	return nil, nil
}

func (m *mockFoodRepoFull) Save(_ context.Context, item *aggregate.FoodItem) error {
	m.items[item.ID()] = item
	return nil
}

func (m *mockFoodRepoFull) Update(_ context.Context, item *aggregate.FoodItem) error {
	m.items[item.ID()] = item
	return nil
}

func (m *mockFoodRepoFull) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *mockFoodRepoFull) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}

func TestCreateAndApproveFoodItemHandler(t *testing.T) {
	t.Parallel()

	repo := &mockFoodRepoFull{items: make(map[string]*aggregate.FoodItem)}

	createHandler := command.NewCreateFoodItemHandler(repo)
	approveHandler := command.NewApproveFoodItemHandler(repo)

	// 1. Create Food Item
	createCmd := command.CreateFoodItemCommand{
		Name:            "Yến mạch Úc",
		Category:        "CARB",
		CaloriesPer100g: 389,
		ProteinPer100g:  16.9,
		CarbsPer100g:    66.3,
		FatPer100g:      6.9,
	}

	created, err := createHandler.Handle(context.Background(), createCmd)
	if err != nil {
		t.Fatalf("unexpected error creating food item: %v", err)
	}
	if got := created.Status(); got != aggregate.FoodStatusPendingApproval {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusPendingApproval)
	}

	// 2. Approve Food Item
	approveCmd := command.ApproveFoodItemCommand{FoodItemID: created.ID()}
	approved, err := approveHandler.Handle(context.Background(), approveCmd)
	if err != nil {
		t.Fatalf("unexpected error approving food item: %v", err)
	}
	if got := approved.Status(); got != aggregate.FoodStatusActive {
		t.Fatalf("got status %q, want %q", got, aggregate.FoodStatusActive)
	}
}
