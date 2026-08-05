package entity

import "time"

type MealLog struct {
	id           string
	mealType     string
	mealName     string
	calories     float64
	protein      float64
	carbs        float64
	fat          float64
	consumedAt   time.Time
	isAIAnalyzed bool
}

func NewMealLog(
	id, mealType, mealName string,
	calories, protein, carbs, fat float64,
	consumedAt time.Time,
	isAIAnalyzed bool,
) MealLog {
	return MealLog{
		id:           id,
		mealType:     mealType,
		mealName:     mealName,
		calories:     calories,
		protein:      protein,
		carbs:        carbs,
		fat:          fat,
		consumedAt:   consumedAt,
		isAIAnalyzed: isAIAnalyzed,
	}
}

func (m MealLog) ID() string            { return m.id }
func (m MealLog) MealType() string      { return m.mealType }
func (m MealLog) MealName() string      { return m.mealName }
func (m MealLog) Calories() float64     { return m.calories }
func (m MealLog) Protein() float64      { return m.protein }
func (m MealLog) Carbs() float64        { return m.carbs }
func (m MealLog) Fat() float64          { return m.fat }
func (m MealLog) ConsumedAt() time.Time { return m.consumedAt }
func (m MealLog) IsAIAnalyzed() bool    { return m.isAIAnalyzed }
