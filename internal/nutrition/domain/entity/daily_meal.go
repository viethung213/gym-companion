package entity

type DailyMeal struct {
	mealType       string
	targetCalories float64
	mealOptions    []MealOption
	consumed       bool
	scheduledTime  string
}

func DefaultScheduledTime(mealType string) string {
	switch mealType {
	case "BREAKFAST":
		return "07:00"
	case "LUNCH":
		return "12:00"
	case "SNACK":
		return "15:30"
	case "DINNER":
		return "19:00"
	default:
		return "12:00"
	}
}

func NewDailyMeal(mealType string, targetCalories float64, mealOptions []MealOption, consumed bool) DailyMeal {
	return NewDailyMealWithSchedule(mealType, targetCalories, mealOptions, consumed, DefaultScheduledTime(mealType))
}

func NewDailyMealWithSchedule(mealType string, targetCalories float64, mealOptions []MealOption, consumed bool, scheduledTime string) DailyMeal {
	optsCopy := make([]MealOption, len(mealOptions))
	copy(optsCopy, mealOptions)

	if scheduledTime == "" {
		scheduledTime = DefaultScheduledTime(mealType)
	}

	return DailyMeal{
		mealType:       mealType,
		targetCalories: targetCalories,
		mealOptions:    optsCopy,
		consumed:       consumed,
		scheduledTime:  scheduledTime,
	}
}

func (m DailyMeal) MealType() string        { return m.mealType }
func (m DailyMeal) TargetCalories() float64 { return m.targetCalories }
func (m DailyMeal) Consumed() bool          { return m.consumed }
func (m DailyMeal) ScheduledTime() string  {
	if m.scheduledTime == "" {
		return DefaultScheduledTime(m.mealType)
	}
	return m.scheduledTime
}

func (m DailyMeal) MealOptions() []MealOption {
	copied := make([]MealOption, len(m.mealOptions))
	copy(copied, m.mealOptions)
	return copied
}

func (m *DailyMeal) MarkAsConsumed() {
	m.consumed = true
}

func (m *DailyMeal) UpdateScheduledTime(t string) {
	if t != "" {
		m.scheduledTime = t
	}
}
