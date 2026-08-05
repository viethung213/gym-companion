package vo

import (
	"strings"
	"time"
)

const (
	LockoutTypeProtein  = "PROTEIN"
	LockoutTypeCarb     = "CARB"
	LockoutTypeCategory = "CATEGORY"

	DurationProtein  = 7 * 24 * time.Hour // 7 days
	DurationCarb     = 5 * 24 * time.Hour // 5 days
	DurationCategory = 3 * 24 * time.Hour // 3 days
)

type LockoutItem struct {
	itemType   string
	itemName   string
	unlockedAt time.Time
}

func NewLockoutItem(itemType, itemName string, unlockedAt time.Time) LockoutItem {
	return LockoutItem{
		itemType:   itemType,
		itemName:   strings.ToUpper(strings.TrimSpace(itemName)),
		unlockedAt: unlockedAt,
	}
}

func (l LockoutItem) ItemType() string      { return l.itemType }
func (l LockoutItem) ItemName() string      { return l.itemName }
func (l LockoutItem) UnlockedAt() time.Time { return l.unlockedAt }
func (l LockoutItem) IsActive(now time.Time) bool {
	return now.Before(l.unlockedAt)
}

type LockoutRegistry struct {
	items []LockoutItem
}

func NewLockoutRegistry(items []LockoutItem) LockoutRegistry {
	itemCopy := make([]LockoutItem, len(items))
	copy(itemCopy, items)
	return LockoutRegistry{items: itemCopy}
}

func (r LockoutRegistry) Items() []LockoutItem { return r.items }

func (r *LockoutRegistry) AddLockout(item LockoutItem) {
	r.items = append(r.items, item)
}

func (r LockoutRegistry) ApplyLockout(itemType, itemName string, duration time.Duration, now time.Time) LockoutRegistry {
	copied := make([]LockoutItem, len(r.items), len(r.items)+1)
	copy(copied, r.items)
	copied = append(copied, NewLockoutItem(itemType, itemName, now.Add(duration)))
	return LockoutRegistry{items: copied}
}

func (r LockoutRegistry) FilterAvailableIngredients(catalog []FoodNutrient, now time.Time) []FoodNutrient {
	activeLockouts := make(map[string]bool)
	for _, l := range r.items {
		if l.IsActive(now) {
			activeLockouts[strings.ToUpper(l.itemName)] = true
		}
	}

	available := make([]FoodNutrient, 0, len(catalog))
	for _, food := range catalog {
		foodNameUpper := strings.ToUpper(food.Name())
		proteinSrcUpper := strings.ToUpper(food.ProteinSource())
		carbSrcUpper := strings.ToUpper(food.CarbSource())

		if activeLockouts[foodNameUpper] || activeLockouts[proteinSrcUpper] || activeLockouts[carbSrcUpper] {
			continue
		}
		available = append(available, food)
	}

	return available
}

func (r LockoutRegistry) CheckCollisions(candidateIngredients []string, now time.Time) ([]string, []string) {
	activeLockouts := make(map[string]bool)
	for _, l := range r.items {
		if l.IsActive(now) {
			activeLockouts[strings.ToUpper(l.itemName)] = true
		}
	}

	available := make([]string, 0, len(candidateIngredients))
	lockedCollisions := make([]string, 0)

	for _, ing := range candidateIngredients {
		ingUpper := strings.ToUpper(strings.TrimSpace(ing))
		if activeLockouts[ingUpper] {
			lockedCollisions = append(lockedCollisions, ing)
		} else {
			available = append(available, ing)
		}
	}

	return available, lockedCollisions
}
