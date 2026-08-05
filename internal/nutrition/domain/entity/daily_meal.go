package entity

type DailyMeal struct {
	mealType       string
	targetCalories float64
	mealOptions    []MealOption
	consumed       bool
}

func NewDailyMeal(mealType string, targetCalories float64, mealOptions []MealOption, consumed bool) DailyMeal {
	optsCopy := make([]MealOption, len(mealOptions))
	copy(optsCopy, mealOptions)

	return DailyMeal{
		mealType:       mealType,
		targetCalories: targetCalories,
		mealOptions:    optsCopy,
		consumed:       consumed,
	}
}

func (m DailyMeal) MealType() string         { return m.mealType }
func (m DailyMeal) TargetCalories() float64   { return m.targetCalories }
func (m DailyMeal) Consumed() bool           { return m.consumed }
func (m DailyMeal) MealOptions() []MealOption {
	copied := make([]MealOption, len(m.mealOptions))
	copy(copied, m.mealOptions)
	return copied
}

func (m *DailyMeal) MarkAsConsumed() {
	m.consumed = true
}
