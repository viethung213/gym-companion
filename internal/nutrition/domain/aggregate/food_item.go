package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
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
	pricePer100g      float64
	costTier          string
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
	pricePer100g float64,
	costTier string,
) (*FoodItem, error) {
	validTier, err := vo.ValidateFoodItem(name, category, pricePer100g, costTier)
	if err != nil {
		return nil, err
	}

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
		pricePer100g:      pricePer100g,
		costTier:          validTier,
		status:            FoodStatusDraft,
		createdAt:         now,
		updatedAt:         now,
	}, nil
}

func ReconstructFoodItem(
	id, name, category string,
	caloriesPer100g, proteinPer100g, carbsPer100g, fatPer100g float64,
	allergenTags []string,
	proteinSource, carbSource string,
	isNutiFoodProduct bool,
	pricePer100g float64,
	costTier string,
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
		pricePer100g:      pricePer100g,
		costTier:          vo.NormalizeCostTier(costTier),
		status:            status,
		createdAt:         createdAt,
		updatedAt:         updatedAt,
	}
}

func (f *FoodItem) ID() string               { return f.id }
func (f *FoodItem) Name() string             { return f.name }
func (f *FoodItem) Category() string         { return f.category }
func (f *FoodItem) CaloriesPer100g() float64 { return f.caloriesPer100g }
func (f *FoodItem) ProteinPer100g() float64  { return f.proteinPer100g }
func (f *FoodItem) CarbsPer100g() float64    { return f.carbsPer100g }
func (f *FoodItem) FatPer100g() float64      { return f.fatPer100g }
func (f *FoodItem) Status() string           { return f.status }
func (f *FoodItem) IsNutiFoodProduct() bool  { return f.isNutiFoodProduct }
func (f *FoodItem) ProteinSource() string    { return f.proteinSource }
func (f *FoodItem) CarbSource() string       { return f.carbSource }
func (f *FoodItem) PricePer100g() float64    { return f.pricePer100g }
func (f *FoodItem) CostTier() string         { return f.costTier }
func (f *FoodItem) SetPricing(pricePer100g float64, costTier string) error {
	validTier, err := vo.ValidateFoodItem(f.name, f.category, pricePer100g, costTier)
	if err != nil {
		return err
	}
	f.pricePer100g = pricePer100g
	f.costTier = validTier
	f.updatedAt = time.Now()
	return nil
}
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
