package aggregate

import (
	"errors"
	"fmt"
	"time"
)

const (
	FoodStatusDraft           = "Draft"
	FoodStatusPendingApproval = "PendingApproval"
	FoodStatusActive          = "Active"
	FoodStatusArchived        = "Archived"
)

var (
	ErrInvalidStatusTransition = errors.New("invalid food item status transition")
)

// FoodItem Aggregate Root managing partner and standard catalog food items.
type FoodItem struct {
	id                string
	name              string
	category          string
	caloriesPer100g   float64
	proteinPer100g    float64
	carbsPer100g      float64
	fatPer100g        float64
	allergenTags      []string
	proteinSource     string
	carbSource        string
	isNutiFoodProduct bool
	status            string
	createdAt         time.Time
	updatedAt         time.Time
}

func NewFoodItem(
	id, name, category string,
	caloriesPer100g, proteinPer100g, carbsPer100g, fatPer100g float64,
	allergenTags []string,
	proteinSource, carbSource string,
	isNutiFoodProduct bool,
) *FoodItem {
	copiedTags := make([]string, len(allergenTags))
	copy(copiedTags, allergenTags)
	now := time.Now()

	return &FoodItem{
		id:                id,
		name:              name,
		category:          category,
		caloriesPer100g:   caloriesPer100g,
		proteinPer100g:    proteinPer100g,
		carbsPer100g:      carbsPer100g,
		fatPer100g:        fatPer100g,
		allergenTags:      copiedTags,
		proteinSource:     proteinSource,
		carbSource:        carbSource,
		isNutiFoodProduct: isNutiFoodProduct,
		status:            FoodStatusDraft,
		createdAt:         now,
		updatedAt:         now,
	}
}

func ReconstructFoodItem(
	id, name, category string,
	caloriesPer100g, proteinPer100g, carbsPer100g, fatPer100g float64,
	allergenTags []string,
	proteinSource, carbSource string,
	isNutiFoodProduct bool,
	status string,
	createdAt, updatedAt time.Time,
) *FoodItem {
	copiedTags := make([]string, len(allergenTags))
	copy(copiedTags, allergenTags)

	return &FoodItem{
		id:                id,
		name:              name,
		category:          category,
		caloriesPer100g:   caloriesPer100g,
		proteinPer100g:    proteinPer100g,
		carbsPer100g:      carbsPer100g,
		fatPer100g:        fatPer100g,
		allergenTags:      copiedTags,
		proteinSource:     proteinSource,
		carbSource:        carbSource,
		isNutiFoodProduct: isNutiFoodProduct,
		status:            status,
		createdAt:         createdAt,
		updatedAt:         updatedAt,
	}
}

func (f *FoodItem) ID() string                { return f.id }
func (f *FoodItem) Name() string              { return f.name }
func (f *FoodItem) Category() string          { return f.category }
func (f *FoodItem) CaloriesPer100g() float64 { return f.caloriesPer100g }
func (f *FoodItem) ProteinPer100g() float64  { return f.proteinPer100g }
func (f *FoodItem) CarbsPer100g() float64    { return f.carbsPer100g }
func (f *FoodItem) FatPer100g() float64      { return f.fatPer100g }
func (f *FoodItem) Status() string          { return f.status }
func (f *FoodItem) IsNutiFoodProduct() bool  { return f.isNutiFoodProduct }
func (f *FoodItem) ProteinSource() string     { return f.proteinSource }
func (f *FoodItem) CarbSource() string        { return f.carbSource }
func (f *FoodItem) AllergenTags() []string {
	copied := make([]string, len(f.allergenTags))
	copy(copied, f.allergenTags)
	return copied
}

func (f *FoodItem) SubmitForApproval() error {
	if f.status != FoodStatusDraft {
		return fmt.Errorf("food item: %w (current status: %s)", ErrInvalidStatusTransition, f.status)
	}
	f.status = FoodStatusPendingApproval
	f.updatedAt = time.Now()
	return nil
}

func (f *FoodItem) Approve() error {
	if f.status != FoodStatusPendingApproval {
		return fmt.Errorf("food item: %w (current status: %s)", ErrInvalidStatusTransition, f.status)
	}
	f.status = FoodStatusActive
	f.updatedAt = time.Now()
	return nil
}

func (f *FoodItem) Reject() error {
	if f.status != FoodStatusPendingApproval {
		return fmt.Errorf("food item: %w (current status: %s)", ErrInvalidStatusTransition, f.status)
	}
	f.status = FoodStatusDraft
	f.updatedAt = time.Now()
	return nil
}

func (f *FoodItem) Archive() error {
	f.status = FoodStatusArchived
	f.updatedAt = time.Now()
	return nil
}
