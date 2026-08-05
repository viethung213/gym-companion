package event

import (
	"time"
)

type NutritionPlanGeneratedEvent struct {
	planID    string
	userID    string
	planDate  time.Time
	timestamp time.Time
}

func NewNutritionPlanGeneratedEvent(planID, userID string, planDate, timestamp time.Time) *NutritionPlanGeneratedEvent {
	return &NutritionPlanGeneratedEvent{
		planID:    planID,
		userID:    userID,
		planDate:  planDate,
		timestamp: timestamp,
	}
}

func (e *NutritionPlanGeneratedEvent) PlanID() string       { return e.planID }
func (e *NutritionPlanGeneratedEvent) UserID() string       { return e.userID }
func (e *NutritionPlanGeneratedEvent) PlanDate() time.Time  { return e.planDate }
func (e *NutritionPlanGeneratedEvent) Timestamp() time.Time { return e.timestamp }

type NutritionPlanRecalibratedEvent struct {
	planID    string
	userID    string
	reason    string
	timestamp time.Time
}

func NewNutritionPlanRecalibratedEvent(planID, userID, reason string, timestamp time.Time) *NutritionPlanRecalibratedEvent {
	return &NutritionPlanRecalibratedEvent{
		planID:    planID,
		userID:    userID,
		reason:    reason,
		timestamp: timestamp,
	}
}

func (e *NutritionPlanRecalibratedEvent) PlanID() string       { return e.planID }
func (e *NutritionPlanRecalibratedEvent) UserID() string       { return e.userID }
func (e *NutritionPlanRecalibratedEvent) Reason() string       { return e.reason }
func (e *NutritionPlanRecalibratedEvent) Timestamp() time.Time { return e.timestamp }

type MealLoggedEvent struct {
	mealLogID string
	userID    string
	mealType  string
	mealName  string
	calories  float64
	loggedAt  time.Time
}

func NewMealLoggedEvent(mealLogID, userID, mealType, mealName string, calories float64, loggedAt time.Time) *MealLoggedEvent {
	return &MealLoggedEvent{
		mealLogID: mealLogID,
		userID:    userID,
		mealType:  mealType,
		mealName:  mealName,
		calories:  calories,
		loggedAt:  loggedAt,
	}
}

func (e *MealLoggedEvent) MealLogID() string   { return e.mealLogID }
func (e *MealLoggedEvent) UserID() string      { return e.userID }
func (e *MealLoggedEvent) MealType() string    { return e.mealType }
func (e *MealLoggedEvent) MealName() string    { return e.mealName }
func (e *MealLoggedEvent) Calories() float64   { return e.calories }
func (e *MealLoggedEvent) LoggedAt() time.Time { return e.loggedAt }

type LockoutAppliedEvent struct {
	userID     string
	itemType   string
	itemName   string
	unlockedAt time.Time
}

func NewLockoutAppliedEvent(userID, itemType, itemName string, unlockedAt time.Time) *LockoutAppliedEvent {
	return &LockoutAppliedEvent{
		userID:     userID,
		itemType:   itemType,
		itemName:   itemName,
		unlockedAt: unlockedAt,
	}
}

func (e *LockoutAppliedEvent) UserID() string        { return e.userID }
func (e *LockoutAppliedEvent) ItemType() string      { return e.itemType }
func (e *LockoutAppliedEvent) ItemName() string      { return e.itemName }
func (e *LockoutAppliedEvent) UnlockedAt() time.Time { return e.unlockedAt }
