package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

var (
	ErrMealOptionNotFound = errors.New("meal option not found in plan")
	ErrPlanAlreadyLogged  = errors.New("meal option has already been logged")
)

type IngredientGram struct {
	ingredientName  string
	grams           float64
	isSupplementary bool
}

func NewIngredientGram(name string, grams float64, isSupplementary bool) IngredientGram {
	return IngredientGram{
		ingredientName:  name,
		grams:           grams,
		isSupplementary: isSupplementary,
	}
}

func (i IngredientGram) IngredientName() string { return i.ingredientName }
func (i IngredientGram) Grams() float64         { return i.grams }
func (i IngredientGram) IsSupplementary() bool  { return i.isSupplementary }

type MealOption struct {
	optionID          string
	mealName          string
	calories          float64
	proteinGrams      float64
	carbGrams         float64
	fatGrams          float64
	ingredients       []IngredientGram
	cookingSteps      []string
	isLogged          bool
	isNutiFoodProduct bool
}

func NewMealOption(
	optionID, mealName string,
	calories, proteinGrams, carbGrams, fatGrams float64,
	ingredients []IngredientGram,
	cookingSteps []string,
	isNutiFoodProduct bool,
) MealOption {
	ingCopy := make([]IngredientGram, len(ingredients))
	copy(ingCopy, ingredients)
	stepCopy := make([]string, len(cookingSteps))
	copy(stepCopy, cookingSteps)

	return MealOption{
		optionID:          optionID,
		mealName:          mealName,
		calories:          calories,
		proteinGrams:      proteinGrams,
		carbGrams:         carbGrams,
		fatGrams:          fatGrams,
		ingredients:       ingCopy,
		cookingSteps:      stepCopy,
		isLogged:          false,
		isNutiFoodProduct: isNutiFoodProduct,
	}
}

func (m *MealOption) OptionID() string        { return m.optionID }
func (m *MealOption) MealName() string        { return m.mealName }
func (m *MealOption) Calories() float64       { return m.calories }
func (m *MealOption) ProteinGrams() float64   { return m.proteinGrams }
func (m *MealOption) CarbGrams() float64      { return m.carbGrams }
func (m *MealOption) FatGrams() float64       { return m.fatGrams }
func (m *MealOption) IsLogged() bool          { return m.isLogged }
func (m *MealOption) IsNutiFoodProduct() bool { return m.isNutiFoodProduct }
func (m *MealOption) Ingredients() []IngredientGram {
	copied := make([]IngredientGram, len(m.ingredients))
	copy(copied, m.ingredients)
	return copied
}
func (m *MealOption) CookingSteps() []string {
	copied := make([]string, len(m.cookingSteps))
	copy(copied, m.cookingSteps)
	return copied
}

type DailyMeal struct {
	mealType      string
	options       []MealOption
	scheduledTime string
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

func NewDailyMeal(mealType string, options []MealOption) DailyMeal {
	return NewDailyMealWithSchedule(mealType, options, DefaultScheduledTime(mealType))
}

func NewDailyMealWithSchedule(mealType string, options []MealOption, scheduledTime string) DailyMeal {
	optCopy := make([]MealOption, len(options))
	copy(optCopy, options)
	if scheduledTime == "" {
		scheduledTime = DefaultScheduledTime(mealType)
	}
	return DailyMeal{
		mealType:      mealType,
		options:       optCopy,
		scheduledTime: scheduledTime,
	}
}

func (d DailyMeal) MealType() string { return d.mealType }
func (d DailyMeal) ScheduledTime() string {
	if d.scheduledTime == "" {
		return DefaultScheduledTime(d.mealType)
	}
	return d.scheduledTime
}

func (d DailyMeal) Options() []MealOption {
	copied := make([]MealOption, len(d.options))
	copy(copied, d.options)
	return copied
}

// NutritionPlan Aggregate Root for daily nutrition planning.
type NutritionPlan struct {
	id                string
	userID            string
	planDate          time.Time
	calorieAllocation vo.CalorieAllocation
	dailyMeals        []DailyMeal
	createdAt         time.Time
	updatedAt         time.Time
}

func NewNutritionPlan(
	id, userID string,
	planDate time.Time,
	allocation vo.CalorieAllocation,
	dailyMeals []DailyMeal,
) *NutritionPlan {
	mealsCopy := make([]DailyMeal, len(dailyMeals))
	copy(mealsCopy, dailyMeals)
	now := time.Now()

	return &NutritionPlan{
		id:                id,
		userID:            userID,
		planDate:          planDate,
		calorieAllocation: allocation,
		dailyMeals:        mealsCopy,
		createdAt:         now,
		updatedAt:         now,
	}
}

func (p *NutritionPlan) ID() string                              { return p.id }
func (p *NutritionPlan) UserID() string                          { return p.userID }
func (p *NutritionPlan) PlanDate() time.Time                     { return p.planDate }
func (p *NutritionPlan) CalorieAllocation() vo.CalorieAllocation { return p.calorieAllocation }
func (p *NutritionPlan) DailyMeals() []DailyMeal {
	copied := make([]DailyMeal, len(p.dailyMeals))
	copy(copied, p.dailyMeals)
	return copied
}
func (p *NutritionPlan) CreatedAt() time.Time { return p.createdAt }
func (p *NutritionPlan) UpdatedAt() time.Time { return p.updatedAt }

func (p *NutritionPlan) MarkOptionLogged(optionID string) (*MealOption, error) {
	for i, meal := range p.dailyMeals {
		for j, opt := range meal.options {
			if opt.optionID == optionID {
				if opt.isLogged {
					return nil, fmt.Errorf("nutrition plan: %w (option: %s)", ErrPlanAlreadyLogged, optionID)
				}
				p.dailyMeals[i].options[j].isLogged = true
				p.updatedAt = time.Now()
				return &p.dailyMeals[i].options[j], nil
			}
		}
	}
	return nil, fmt.Errorf("nutrition plan: %w (option: %s)", ErrMealOptionNotFound, optionID)
}

func (p *NutritionPlan) UpdateRemainingUnconsumedMeals(updatedMeals []DailyMeal, newAllocation vo.CalorieAllocation) {
	p.calorieAllocation = newAllocation

	for i, existingMeal := range p.dailyMeals {
		var hasLoggedOption bool
		for _, opt := range existingMeal.options {
			if opt.isLogged {
				hasLoggedOption = true
				break
			}
		}

		if !hasLoggedOption {
			for _, updatedMeal := range updatedMeals {
				if updatedMeal.mealType == existingMeal.mealType {
					p.dailyMeals[i] = updatedMeal
					break
				}
			}
		}
	}
	p.updatedAt = time.Now()
}

func (p *NutritionPlan) UpdateMealSchedule(schedules map[string]string) {
	for i, meal := range p.dailyMeals {
		if newTime, ok := schedules[meal.mealType]; ok && newTime != "" {
			p.dailyMeals[i].scheduledTime = newTime
		}
	}
	p.updatedAt = time.Now()
}
