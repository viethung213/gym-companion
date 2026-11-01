package seeder_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
)

type mockFoodRegistrar struct {
	mu           sync.Mutex
	createCalls  []*nutritionv1msg.CreateFoodItemRequest
	approveCalls []*nutritionv1msg.ApproveFoodItemRequest
	errToRet     error
	counter      int
}

func (m *mockFoodRegistrar) CreateFoodItem(ctx context.Context, req *nutritionv1msg.CreateFoodItemRequest) (*nutritionv1msg.CreateFoodItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.counter++
	m.createCalls = append(m.createCalls, req)
	return &nutritionv1msg.CreateFoodItemResponse{
		FoodItemId: fmt.Sprintf("food-%d", m.counter),
		Status:     "PendingApproval",
		Message:    "OK",
	}, nil
}

func (m *mockFoodRegistrar) ApproveFoodItem(ctx context.Context, req *nutritionv1msg.ApproveFoodItemRequest) (*nutritionv1msg.ApproveFoodItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.approveCalls = append(m.approveCalls, req)
	return &nutritionv1msg.ApproveFoodItemResponse{
		FoodItemId: req.GetFoodItemId(),
		Status:     "Active",
		Message:    "Approved",
	}, nil
}

func TestSeedFoodItems_NilService(t *testing.T) {
	err := seeder.SeedFoodItems(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when foodService is nil, got nil")
	}
}

func TestSeedFoodItems_Success(t *testing.T) {
	mock := &mockFoodRegistrar{}
	ctx := context.Background()

	err := seeder.SeedFoodItems(ctx, nil, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.createCalls) == 0 {
		t.Fatalf("expected create calls, got 0")
	}
	if len(mock.approveCalls) != len(mock.createCalls) {
		t.Errorf("expected approve calls %d to match create calls %d", len(mock.approveCalls), len(mock.createCalls))
	}

	// Verify categories
	hasProtein := false
	hasCarb := false
	hasNutifood := false

	for _, item := range mock.createCalls {
		if item.Category == "PROTEIN" {
			hasProtein = true
		}
		if item.Category == "CARB" {
			hasCarb = true
		}
		if item.IsNutifoodProduct {
			hasNutifood = true
		}
	}

	if !hasProtein {
		t.Errorf("expected some protein items")
	}
	if !hasCarb {
		t.Errorf("expected some carb items")
	}
	if !hasNutifood {
		t.Errorf("expected some nutifood items")
	}
}

func TestSeedFoodItems_ContextCancellation(t *testing.T) {
	mock := &mockFoodRegistrar{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := seeder.SeedFoodItems(ctx, nil, mock)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
